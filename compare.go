package main

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func compareCommand() *cobra.Command {
	var (
		oldFile string
		newFile string

		opts *metricOptions
	)

	c := &cobra.Command{
		Use:   "compare",
		Short: "Compare takes two metrics files and compares them takes all the metrics and outputs them as key value pairs. It will average histograms",
		RunE: func(c *cobra.Command, _ []string) error {
			if oldFile == "" {
				return errors.New("old-file must be specified")
			}
			if newFile == "" {
				return errors.New("new-file must be specified")
			}
			oldFamilies, err := readFile(oldFile)
			if err != nil {
				return errors.Wrap(err, "error reading old file")
			}

			oldMetricMap, err := familiesToKeyPairs(oldFamilies, opts)
			if err != nil {
				return errors.Wrap(err, "error generating old metric map")
			}

			newFamilies, err := readFile(newFile)
			if err != nil {
				return errors.Wrap(err, "error reading new file")
			}

			newMetricMap, err := familiesToKeyPairs(newFamilies, opts)
			if err != nil {
				return errors.Wrap(err, "error generating new metric map")
			}

			compareMetricMaps(oldMetricMap, newMetricMap)
			return nil
		},
	}

	c.Flags().StringVar(&oldFile, "old-file", "", "old metrics file to parse")
	c.Flags().StringVar(&newFile, "new-file", "", "new metrics file to parse")

	opts = addMetricFlags(c)

	return c
}

func compareMetricMaps(oldMap, newMap metricMap) {
	var keys []familyKey
	for k := range oldMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].metric != keys[j].metric {
			return keys[i].metric < keys[j].metric
		}
		return keys[i].labels < keys[j].labels
	})

	// Show comparisons
	for _, k := range keys {
		if _, ok := newMap[k]; !ok {
			continue
		}

		if oldMap[k].value != 0 {
			percentChange := (newMap[k].value - oldMap[k].value) / oldMap[k].value * 100
			fmt.Printf("%s %s (old: %s, new %s): change: %0.4f%%\n", k.metric, k.labels, oldMap[k].String(), newMap[k].String(), percentChange)
		} else {
			fmt.Printf("%s %s (old: %s, new %s)\n", k.metric, k.labels, oldMap[k].String(), newMap[k].String())
		}
	}
}
