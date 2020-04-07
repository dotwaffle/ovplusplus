package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/dotwaffle/ovplusplus/pkg/irr"
	"github.com/dotwaffle/ovplusplus/pkg/pfxops"
	"github.com/dotwaffle/ovplusplus/pkg/rpki"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// serveCmd implements the serve command.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve an export.json file based on IRR and RPKI data.",
	Run: func(cmd *cobra.Command, args []string) {
		var export string
		var mu sync.Mutex
		routes := make(map[string][]irr.Route)
		var roas []rpki.ROA
		updated := make(chan bool)
		defer close(updated)

		go func() {
			ticker := time.NewTicker(viper.GetDuration("refresh"))
			defer ticker.Stop()
			for {
				ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("refresh"))
				defer cancel()
				e, eCtx := errgroup.WithContext(ctx)

				var newMu sync.Mutex
				newRoutes := make(map[string][]irr.Route)

				// for each input, get the data
				for _, src := range viper.GetStringSlice("file") {
					src := src
					e.Go(func() error {
						srcRoutes, err := irr.FetchFile(eCtx, src)
						if err != nil {
							return err
						}
						log.Debug().Int("routes", len(srcRoutes)).Str("src", src).Msg("irrdb parsed")
						newMu.Lock()
						newRoutes[src] = srcRoutes
						newMu.Unlock()
						return nil
					})
				}

				for _, src := range viper.GetStringSlice("irrdb") {
					src := src
					e.Go(func() error {
						srcRoutes, err := irr.FetchURL(eCtx, src)
						if err != nil {
							return err
						}
						log.Debug().Int("routes", len(srcRoutes)).Str("src", src).Msg("irrdb parsed")
						newMu.Lock()
						newRoutes[src] = srcRoutes
						newMu.Unlock()
						return nil
					})
				}

				if err := e.Wait(); err != nil {
					log.Error().Err(err).Msg("irrdb read")
					continue
				}

				mu.Lock()
				routes = newRoutes
				mu.Unlock()
				updated <- true

				// wait for tick
				<-ticker.C
			}
		}()

		go func() {
			ticker := time.NewTicker(viper.GetDuration("refresh"))
			defer ticker.Stop()
			for {
				ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("refresh"))
				defer cancel()

				// import RPKI ROA export
				newROAs, err := rpki.Fetch(ctx, viper.GetString("rpki"))
				if err != nil {
					log.Error().Err(err).Msg("rpki.Fetch()")
					continue
				}

				mu.Lock()
				roas = newROAs
				log.Debug().Int("roas", len(roas)).Msg("rpki parsed")
				mu.Unlock()
				updated <- true

				<-ticker.C
			}
		}()

		go func() {
			for range updated {
				mu.Lock()
				// merge data
				results, err := pfxops.Merge(roas, routes)
				if err != nil {
					log.Error().Err(err).Msg("pfxops.Merge()")
					mu.Unlock()
					continue
				}
				sort.SliceStable(results, func(i, j int) bool {
					switch {
					case results[i].ASN < results[j].ASN:
						return true
					case results[i].ASN > results[j].ASN:
						return false
					case results[i].MaxLength < results[j].MaxLength:
						return true
					case results[i].MaxLength > results[j].MaxLength:
						return false
					case results[i].Prefix < results[j].Prefix:
						return true
					case results[i].Prefix > results[j].Prefix:
						return false
					case results[i].TA < results[j].TA:
						return true
					case results[i].TA > results[j].TA:
						return false
					default:
						return false
					}
				})
				log.Debug().Int("roas", len(results)).Msg("new total roas")

				// prepare the output
				output, err := json.MarshalIndent(rpki.Export{ROAs: results}, "", "\t")
				if err != nil {
					log.Error().Err(err).Msg("json.Marshal()")
					mu.Unlock()
					continue
				}
				export = string(output)
				mu.Unlock()
			}
		}()

		// serve the data
		httpExport := func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprintf(w, export)
		}
		http.HandleFunc("/export.json", httpExport)
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal().Err(err).Msg("http.ListenAndServe()")
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// fetch IRR data from a URL
	serveCmd.Flags().StringSliceP("irrdb", "i", []string{}, "url to fetch containing IRRDB data")
	if err := viper.BindPFlag("irrdb", serveCmd.Flags().Lookup("irrdb")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): irrdb")
	}

	// fetch IRR data from a local file
	serveCmd.Flags().StringSliceP("file", "f", []string{}, "local file containing IRRDB data")
	if err := viper.BindPFlag("file", serveCmd.Flags().Lookup("file")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): file")
	}

	// fetch RPKI data from a URL
	serveCmd.Flags().StringP("rpki", "r", "", "url to fetch containing RPKI ROA data")
	if err := viper.BindPFlag("rpki", serveCmd.Flags().Lookup("rpki")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): rpki")
	}

	// refresh interval
	serveCmd.Flags().DurationP("refresh", "R", time.Hour, "interval between refreshing external data")
	if err := viper.BindPFlag("refresh", serveCmd.Flags().Lookup("refresh")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): refresh")
	}
}
