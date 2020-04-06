package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/dotwaffle/ovplusplus/pkg/irr"
	"github.com/dotwaffle/ovplusplus/pkg/pfxops"
	"github.com/dotwaffle/ovplusplus/pkg/rpki"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

// mergeCmd implements the irr command.
var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Create an export.json file based on IRR and RPKI data.",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.TODO()
		var mu sync.RWMutex
		routes := make(map[string][]irr.Route)
		e, eCtx := errgroup.WithContext(ctx)

		// for each input, get the data
		for _, src := range viper.GetStringSlice("file") {
			src := src
			e.Go(func() error {
				srcRoutes, err := irr.FetchFile(eCtx, src)
				if err != nil {
					return err
				}
				log.Debug().Int("routes", len(srcRoutes)).Str("src", src).Msg("irrdb parsed")
				mu.Lock()
				routes[src] = srcRoutes
				mu.Unlock()
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
				mu.Lock()
				routes[src] = srcRoutes
				mu.Unlock()
				return nil
			})
		}

		if err := e.Wait(); err != nil {
			log.Fatal().Err(err).Msg("irrdb read")
		}

		// produce some stats
		merged := make(map[string][]string)
		for _, r := range routes {
			for _, rr := range r {
				route := rr.Prefix.String()
				merged[route] = append(merged[route], rr.Origin)
			}
		}
		log.Debug().Int("routes", len(merged)).Msg("irrdb parsed total")
		mergedStats := make(map[int]int)
		for _, v := range merged {
			mergedStats[len(v)]++
		}
		depth := make([]int, 0, len(mergedStats))
		for k := range mergedStats {
			depth = append(depth, k)
		}
		sort.Ints(depth)
		for _, k := range depth {
			log.Debug().Int("depth", k).Int("count", mergedStats[k]).Msg("irrdb depth stats")
		}

		// import RPKI ROA export
		roas, err := rpki.Fetch(ctx, viper.GetString("rpki"))
		if err != nil {
			log.Fatal().Err(err).Msg("rpki.Fetch()")
		}
		log.Debug().Int("roas", len(roas)).Msg("rpki parsed")

		// merge data
		results, err := pfxops.Merge(roas, routes)
		if err != nil {
			log.Fatal().Err(err).Msg("pfxops.Merge()")
		}
		sort.SliceStable(results, func(i, j int) bool { return results[i].Prefix < results[j].Prefix })
		log.Debug().Int("roas", len(results)).Msg("new total roas")

		// dump the output to stdout
		output, err := json.Marshal(rpki.Export{ROAs: results})
		if err != nil {
			log.Fatal().Err(err).Msg("json.Marshal()")
		}
		fmt.Println(string(output))
	},
}

func init() {
	rootCmd.AddCommand(mergeCmd)

	// fetch IRR data from a URL
	mergeCmd.Flags().StringSliceP("irrdb", "i", []string{}, "url to fetch containing IRRDB data")
	if err := viper.BindPFlag("irrdb", mergeCmd.Flags().Lookup("irrdb")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): irrdb")
	}

	// fetch IRR data from a local file
	mergeCmd.Flags().StringSliceP("file", "f", []string{}, "local file containing IRRDB data")
	if err := viper.BindPFlag("file", mergeCmd.Flags().Lookup("file")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): file")
	}

	// fetch RPKI data from a URL
	mergeCmd.Flags().StringP("rpki", "r", "", "url to fetch containing RPKI ROA data")
	if err := viper.BindPFlag("rpki", mergeCmd.Flags().Lookup("rpki")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): rpki")
	}
	// mergeCmd.MarkFlagRequired("rpki")

	// use "orlonger" semantics instead of "exact" matching
	mergeCmd.Flags().BoolP("longer", "l", false, "accept longer prefixes as well")
	if err := viper.BindPFlag("longer", mergeCmd.Flags().Lookup("longer")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): longer")
	}
}
