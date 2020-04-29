package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func compareCommand() *cobra.Command {
	var (
		oldFile string
		newFile string
		metrics string
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
				return fmt.Errorf("error reading old file: %v", err)
			}

			oldMetricMap, err := familiesToKeyPairs(oldFamilies)
			if err != nil {
				return fmt.Errorf("error generating old metric map: %v", err)
			}

			newFamilies, err := readFile(newFile)
			if err != nil {
				return fmt.Errorf("error reading new file: %v", err)
			}

			newMetricMap, err := familiesToKeyPairs(newFamilies)
			if err != nil {
				return fmt.Errorf("error generating new metric map: %v", err)
			}

			var metricSlice []string
			if metrics != "" {
				metricSlice = strings.Split(metrics, ",")
			}

			compareMetricMaps(oldMetricMap, newMetricMap, metricSlice)
			return nil
		},
	}

	c.Flags().StringVar(&oldFile, "old-file", "", "old metrics file to parse")
	c.Flags().StringVar(&newFile, "new-file", "", "new metrics file to parse")
	c.Flags().StringVar(&metrics, "metrics", "", "comma separate list of metrics to include in the results")

	return c
}

func compareMetricMaps(oldMap, newMap metricMap, metricSlice []string) {
	desiredMetrics := make(map[string]struct{})
	for _, m := range metricSlice {
		desiredMetrics[m] = struct{}{}
	}

	fmt.Printf("Metrics: %+v\n", desiredMetrics)

	var removedKeys []familyKey

	// Show comparisons
	for k := range oldMap {
		if _, ok := newMap[k]; !ok {
			removedKeys = append(removedKeys, k)
			continue
		}

		if len(desiredMetrics) > 0 {
			if _, ok := desiredMetrics[k.metric]; !ok {
				continue
			}
		}

		if oldMap[k].value != 0 {
			percentChange := (newMap[k].value - oldMap[k].value) / oldMap[k].value * 100
			fmt.Printf("%s %s (old: %s, new %s): change: %0.4f%%\n", k.metric, k.labels, oldMap[k].String(), newMap[k].String(), percentChange)
		} else {
			fmt.Printf("%s %s (old: %s, new %s)\n", k.metric, k.labels, oldMap[k].String(), newMap[k].String())
		}
	}

	fmt.Println()
	for _, removed := range removedKeys {
		fmt.Println("Removed", removed)
	}

	//for k, v := range oldMap {
	//
	//}
}
