package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type changeThresholds struct {
	warnAt  float64
	errorAt float64
}

func compareCommand() *cobra.Command {
	var (
		oldFile string
		newFile string
		warnAt  float64
		errorAt float64

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

			compareMetricMaps(oldMetricMap, newMetricMap, changeThresholds{warnAt, errorAt}, opts)
			return nil
		},
	}

	c.Flags().StringVar(&oldFile, "old-file", "", "old metrics file to parse")
	c.Flags().StringVar(&newFile, "new-file", "", "new metrics file to parse")
	c.Flags().Float64Var(&warnAt, "warn", 0, "warn when values change more or less than this percentage amount")
	c.Flags().Float64Var(&errorAt, "error", 0, "error when values change more or less than this percentage amount and exit 1")

	opts = addMetricFlags(c)

	return c
}

func stdoutPrint(keys []familyKey, oldMap, newMap metricMap, deltas oldNewDeltaMap) {
	// Show comparisons
	for _, k := range keys {
		delta := deltas[k]
		if oldMap[k].value != 0 {
			prefixParts := make([]string, 0)
			if delta.isWarn || delta.isError {
				prefixParts = append(prefixParts, "1")
			}
			if delta.isError {
				prefixParts = append(prefixParts, "31", "4")
			}
			decorationPrefix := ""
			if len(prefixParts) > 0 {
				decorationPrefix = "\033[" + strings.Join(prefixParts, ";") + "m"
			}
			decorationSuffix := ""
			if decorationPrefix != "" {
				decorationSuffix = "\033[0m"
			}
			fmt.Printf("%s%s %s (old: %s, new %s): change: %0.4f%%%s\n",
				decorationPrefix, k.metric, k.labels, oldMap[k].String(), newMap[k].String(), delta.percentChange, decorationSuffix)
		} else {
			fmt.Printf("%s %s (old: %s, new %s)\n", k.metric, k.labels, oldMap[k].String(), newMap[k].String())
		}
	}
}

func csvPrint(keys []familyKey, oldMap, newMap metricMap, deltas oldNewDeltaMap) {
	for _, k := range keys {
		newMetric := newMap[k]
		oldMetric := oldMap[k]
		delta := deltas[k]
		if oldMetric.value != 0 {
			fmt.Printf("%s,%s,%g,%g,%g\n", k.metric, k.labels, oldMetric.value, newMetric.value, delta.percentChange)
			//fmt.Printf("%s %s (old: %s, new %s): change: %0.4f%%\n", k.metric, k.labels, oldMap[k].String(), newMap[k].String(), percentChange)
		} else {
			fmt.Printf("%s,%s,%g,%g,N/A\n", k.metric, k.labels, oldMetric.value, newMetric.value)
		}
	}
}

type oldNewDelta struct {
	percentChange float64
	isWarn        bool
	isError       bool
}

type oldNewDeltaMap map[familyKey]oldNewDelta

func getDeltas(keys []familyKey, oldMap, newMap metricMap, thresholds changeThresholds) oldNewDeltaMap {
	deltas := make(oldNewDeltaMap)
	for _, k := range keys {
		newMetric := newMap[k]
		oldMetric := oldMap[k]
		if oldMetric.value != 0 {
			percentChange := (newMetric.value - oldMetric.value) / oldMetric.value * 100
			deltas[k] = oldNewDelta{
				percentChange: percentChange,
				isError:       thresholds.errorAt != 0.0 && math.Abs(percentChange) > thresholds.errorAt,
				isWarn:        thresholds.warnAt != 0.0 && math.Abs(percentChange) > thresholds.warnAt,
			}
		} else {
			deltas[k] = oldNewDelta{
				isWarn:  false,
				isError: false,
			}
		}
	}

	return deltas
}

func compareMetricMaps(oldMap, newMap metricMap, thresholds changeThresholds, opts *metricOptions) {
	var keys []familyKey
	for k := range oldMap {
		if _, ok := newMap[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].metric != keys[j].metric {
			return keys[i].metric < keys[j].metric
		}
		return keys[i].labels < keys[j].labels
	})

	var deltas = getDeltas(keys, oldMap, newMap, thresholds)

	switch opts.format {
	case "plain":
		stdoutPrint(keys, oldMap, newMap, deltas)
	case "csv":
		csvPrint(keys, oldMap, newMap, deltas)
	default:
		panic("unknown output format")
	}

	for _, v := range deltas {
		if v.isError {
			os.Exit(1)
		}
	}
}
