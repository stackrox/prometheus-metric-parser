package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/prom2json"
	"github.com/spf13/cobra"
)

type metricOptions struct {
	metrics           string
	minHistogramCount int
	trimPrefix        string
	format            string
}

func addMetricFlags(c *cobra.Command) *metricOptions {
	var opts metricOptions
	c.Flags().StringVar(&opts.metrics, "metrics", "", "comma separate list of metrics to include in the results")
	c.Flags().IntVar(&opts.minHistogramCount, "min-histogram-counts", 5, "minimum number of histogram counts to be completed in order for the metric to show up")
	c.Flags().StringVar(&opts.trimPrefix, "trim-prefix-histogram-counts", "rox_central_", "prefix to automatically strip")
	c.Flags().StringVar(&opts.format, "format", "plain", "format to output the metrics in (options are plain or csv)")
	return &opts
}

type labelPair map[string]string

func (l labelPair) String() string {
	var keys []string
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sb := strings.Builder{}
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%s ", k, l[k]))
	}
	return strings.TrimSpace(sb.String())
}

type metric struct {
	value      float64
	sum, count float64
}

func (m metric) String() string {
	if m.count != 0 {
		return fmt.Sprintf("(%0.2f/%d) %0.2f", m.sum, int64(m.count), m.value)
	}
	return fmt.Sprintf("%0.2f", m.value)
}

type metricMap map[familyKey]metric

func (m metricMap) toSortedKeys() []familyKey {
	var keys []familyKey
	for k := range m {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].metric != keys[j].metric {
			return keys[i].metric < keys[j].metric
		}
		return keys[i].labels < keys[j].labels
	})
	return keys
}

func (m metricMap) stdout(keys []familyKey) {
	// Longest key with +1 padding
	var keyStrings []string
	var longest int
	for _, k := range keys {
		stringKey := fmt.Sprintf("%s %s", k.metric, k.labels)
		if len(stringKey) > longest {
			longest = len(stringKey)
		}
		keyStrings = append(keyStrings, stringKey)
	}

	for i, k := range keys {
		keyString := keyStrings[i]
		if m[k].count == 0 {
			fmt.Printf("%-80v %0.0f\n", keyString, m[k].value)
		} else {
			fraction := fmt.Sprintf("(%0.0f/%d)", m[k].sum, int64(m[k].count))
			fmt.Printf("%-80v %s %0.3f\n", keyString, fraction, m[k].value)
		}
	}
}

func (m metricMap) csv(keys []familyKey) {
	for _, k := range keys {
		fmt.Printf("%s,%s,%g\n", k.metric, k.labels, m[k].value)
	}
}

func (m metricMap) Print(format string) {
	keys := m.toSortedKeys()

	switch format {
	case "plain":
		m.stdout(keys)
	case "csv":
		m.csv(keys)
	default:
		panic("unknown format")
	}
}

type familyKey struct {
	metric string
	labels string
}

func familiesToKeyPairs(families []*prom2json.Family, opts *metricOptions) (metricMap, error) {
	desiredMetrics := make(map[string]struct{})
	if opts.metrics != "" {
		for _, m := range strings.Split(opts.metrics, ",") {
			desiredMetrics[m] = struct{}{}
		}
	}

	metricMap := make(map[familyKey]metric)
	for _, family := range families {
		if len(desiredMetrics) > 0 {
			if _, ok := desiredMetrics[family.Name]; !ok {
				continue
			}
		}

		switch family.Type {
		case "HISTOGRAM":
			for _, familyMetric := range family.Metrics {
				histogram := familyMetric.(prom2json.Histogram)
				count, err := strconv.ParseFloat(histogram.Count, 64)
				if err != nil {
					return nil, err
				}
				if count < float64(opts.minHistogramCount) {
					continue
				}

				sum, err := strconv.ParseFloat(histogram.Sum, 64)
				if err != nil {
					return nil, err
				}

				metricMap[familyKey{
					metric: strings.TrimPrefix(family.Name, opts.trimPrefix),
					labels: labelPair(histogram.Labels).String(),
				}] = metric{
					value: sum / count,
					sum:   sum,
					count: count,
				}
			}
		case "COUNTER", "GAUGE":
			for _, familyMetric := range family.Metrics {
				m := familyMetric.(prom2json.Metric)
				value, err := strconv.ParseFloat(m.Value, 64)
				if err != nil {
					return nil, err
				}
				metricMap[familyKey{
					metric: strings.TrimPrefix(family.Name, opts.trimPrefix),
					labels: labelPair(m.Labels).String(),
				}] = metric{
					value: value,
				}
			}
		case "SUMMARY":
		default:
			log.Printf("Unknown family: %+v", family)
		}
	}
	return metricMap, nil
}
