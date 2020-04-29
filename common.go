package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/prom2json"
)

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
	return sb.String()
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

func (m metricMap) Print() {
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
	for _, k := range keys {
		if m[k].count == 0 {
			fmt.Printf("%s %s %0.3f\n", k.metric, k.labels, m[k].value)
		} else {
			fmt.Printf("%s %s (%0.3f/%d) %0.3f\n", k.metric, k.labels, m[k].sum, int64(m[k].count), m[k].value)
		}
	}
}

type familyKey struct {
	metric string
	labels string
}

func familiesToKeyPairs(families []*prom2json.Family) (metricMap, error) {
	metricMap := make(map[familyKey]metric)
	for _, family := range families {
		switch family.Type {
		case "HISTOGRAM":
			for _, familyMetric := range family.Metrics {
				histogram := familyMetric.(prom2json.Histogram)
				count, err := strconv.ParseFloat(histogram.Count, 64)
				if err != nil {
					return nil, err
				}
				sum, err := strconv.ParseFloat(histogram.Sum, 64)
				if err != nil {
					return nil, err
				}
				metricMap[familyKey{
					metric: family.Name,
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
					metric: family.Name,
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
