package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/prom2json"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
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
	c.Flags().StringVar(&opts.format, "format", "plain", "format to output the metrics in (options are plain, csv, influxdb or gcp-monitoring)")
	c.Flags().StringVar(&opts.labels, "labels", "", "comma separate list of labels to include in ingest e.g. Test=ci-scale-test,ClusterFlavor=gke")
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
			fmt.Fprintf(out, "%-80v %0.0f\n", keyString, m[k].value)
		} else {
			fraction := fmt.Sprintf("(%0.0f/%d)", m[k].sum, int64(m[k].count))
			fmt.Fprintf(out, "%-80v %s %0.3f\n", keyString, fraction, m[k].value)
		}
	}
}

func (m metricMap) csv(keys []familyKey, labels map[string]string) {
	w := csv.NewWriter(out)
	header := []string{"metric", "labels", "value"}
	additionalHeaders := maps.Keys(labels)
	slices.Sort(additionalHeaders)
	header = append(header, additionalHeaders...)
	additionalValues := make([]string, 0, len(additionalHeaders))
	for _, h := range additionalHeaders {
		additionalValues = append(additionalValues, labels[h])
	}
	if err := w.Write(header); err != nil {
		log.Fatalln("error writing header to csv:", err)
	}
	for _, k := range keys {
		value := fmt.Sprintf("%.8f", m[k].value)
		record := []string{k.metric, k.labels, value}
		record = append(record, additionalValues...)
		if err := w.Write(record); err != nil {
			log.Fatalln("error writing record to csv:", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		log.Fatalln("error flushing to csv:", err)
	}
}

func (m metricMap) printInfluxDBLineProtocol(keys []familyKey, labels map[string]string, timestamp int64) {
	for _, k := range keys {
		influxStr := k.metric
		influxStr += makeInfluxdbLabels(m[k].labels)
		influxStr += makeInfluxdbLabels(labels)
		fmt.Fprintf(out, "%s value=%g %d\n", influxStr, m[k].value, timestamp)
	}
}

func makeInfluxdbLabels(labels map[string]string) string {
	labelsStr := ""
	for labelKey, labelValue := range labels {
		labelsStr += "," + labelKey + "=" + strings.ReplaceAll(labelValue, " ", "\\ ")
	}
	return labelsStr
}

func (m metricMap) writeToGoogleCloudMonitoring(keys []familyKey, labels map[string]string, projectID string, timestamp int64) {
	fmt.Print("Writing metrics")
	g, err := gcpMonitoringConnect(projectID)
	defer g.close()
	if err != nil {
		log.Fatalf("Cannot connect to GCP monitoring: %v\n", err)
	}
	errorCount := 0
	for _, v := range m {
		err := g.writeTimeSeriesValue(v, labels, timestamp)
		if err != nil {
			log.Println(errors.Wrap(err, "error writing metric: "+v.name))
			errorCount++
		}
		fmt.Print(".")
	}
	// This is a way to calculate 5% to avoid division/multiplying float numbers.
	// Divide both sides by 20 * len(m) and you'll get:
	// 0.05 < errorCount / len(m)
	if len(m) < 20*errorCount {
		log.Fatal("More than 5% of GCP requests failed. Exiting the job.")
	}
	fmt.Println("done")
}

var out io.Writer = os.Stdout

func (m metricMap) Print(format string, labels map[string]string, projectID string, timestamp int64) {
	keys := m.toSortedKeys()

	switch format {
	case "plain":
		m.stdout(keys)
	case "csv":
		m.csv(keys, labels)
	case "influxdb":
		m.printInfluxDBLineProtocol(keys, labels, timestamp)
	case "gcp-monitoring":
		m.writeToGoogleCloudMonitoring(keys, labels, projectID, timestamp)
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
