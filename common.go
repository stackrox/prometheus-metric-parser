package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/prom2json"
	"github.com/spf13/cobra"
)

type metricOptions struct {
	metrics           string
	minHistogramCount int
	trimPrefix        string
	format            string
	labels            string
	projectID         string
	timestamp         int64
}

func addMetricFlags(c *cobra.Command) *metricOptions {
	var opts metricOptions
	c.Flags().StringVar(&opts.metrics, "metrics", "", "comma separate list of metrics to include in the results")
	c.Flags().IntVar(&opts.minHistogramCount, "min-histogram-counts", 5, "minimum number of histogram counts to be completed in order for the metric to show up")
	c.Flags().StringVar(&opts.trimPrefix, "trim-prefix-histogram-counts", "rox_central_", "prefix to automatically strip")
	c.Flags().StringVar(&opts.format, "format", "plain", "format to output the metrics in (options are plain, csv, influxdb or gcloud)")
	c.Flags().StringVar(&opts.labels, "labels", "", "comma separate list of labels to include in injest e.g. Test=ci-scale-test,ClusterFlavor=gke")
	c.Flags().StringVar(&opts.projectID, "project-id", "", "where to send the metrics e.g. stackrox-ci")
	c.Flags().Int64Var(&opts.timestamp, "timestamp", 0, "seconds since the epoch UTC")
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
	name       string
	labels     map[string]string
	value      float64
	sum, count float64
	family     *prom2json.Family
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

func (m metricMap) influxdb(keys []familyKey, labels map[string]string, timestamp int64) {
	for _, k := range keys {
		influxStr := k.metric
		for labelKey, labelValue := range m[k].labels {
			influxStr += "," + labelKey + "=" + strings.ReplaceAll(labelValue, " ", "\\ ")
		}
		for labelKey, labelValue := range labels {
			influxStr += "," + labelKey + "=" + strings.ReplaceAll(labelValue, " ", "\\ ")
		}
		fmt.Printf("%s value=%g %d\n", influxStr, m[k].value, timestamp)
	}
}

func (m metricMap) gcloud(keys []familyKey, labels map[string]string, projectID string, timestamp int64) {
	fmt.Print("Writing metrics")
	g, err := gcloudConnect(projectID)
	if err != nil {
		log.Fatalf("Cannot connect to gcloud: %v\n", err)
	}
	for _, v := range m {
		err := g.writeGcloudTimeSeriesValue(v, labels, timestamp)
		if err != nil {
			log.Fatal(errors.Wrap(err, "error writing metric: "+v.name))
		}
		fmt.Print(".")
	}
	g.close()
	fmt.Println("done")
}

func (m metricMap) Print(format string, labels map[string]string, projectID string, timestamp int64) {
	keys := m.toSortedKeys()

	switch format {
	case "plain":
		m.stdout(keys)
	case "csv":
		m.csv(keys)
	case "influxdb":
		m.influxdb(keys, labels, timestamp)
	case "gcloud":
		m.gcloud(keys, labels, projectID, timestamp)
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

		metricName := strings.TrimPrefix(family.Name, opts.trimPrefix)

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
					metric: metricName,
					labels: labelPair(histogram.Labels).String(),
				}] = metric{
					name:   metricName,
					labels: histogram.Labels,
					value:  sum / count,
					sum:    sum,
					count:  count,
					family: family,
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
					metric: metricName,
					labels: labelPair(m.Labels).String(),
				}] = metric{
					name:   metricName,
					labels: m.Labels,
					value:  value,
					family: family,
				}
			}
		case "SUMMARY":
		default:
			log.Printf("Unknown family: %+v", family)
		}
	}
	return metricMap, nil
}

func labelsFromOpts(optLabels string) map[string]string {
	labels := make(map[string]string)
	for _, pair := range strings.Split(optLabels, ",") {
		x := strings.Split(pair, "=")
		if len(x) == 2 {
			name := strings.TrimSpace(x[0])
			value := strings.TrimSpace(x[1])
			if name != "" && value != "" {
				labels[name] = value
			}
		}
	}

	return labels
}
