/* Copyright 2020 Joeri Hermans, Victor Penso, Matteo Dessalvi

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>. */

package main

import (
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// JSON structures for parsing sinfo output
type SinfoResponse struct {
	Sinfo []SinfoNode `json:"sinfo"`
}

type SinfoNode struct {
	Name  string   `json:"name"`
	Gres  GresInfo `json:"gres"`
	State []string `json:"state"`
}

type GresInfo struct {
	Total string `json:"total"`
	Used  string `json:"used"`
}

type GPUsMetrics struct {
	alloc       float64
	idle        float64
	other       float64
	total       float64
	utilization float64
}

func GPUsGetMetrics() *GPUsMetrics {
	return ParseGPUsMetrics()
}

/* TODO:
  sinfo has gresUSED since slurm>=19.05.0rc01 https://github.com/SchedMD/slurm/blob/master/NEWS
  revert to old process on slurm<19.05.0rc01
  --format=AllocGRES will return gres/gpu=8
  --format=AllocTRES will return billing=16,cpu=16,gres/gpu=8,mem=256G,node=1
func ParseAllocatedGPUs() float64 {
	var num_gpus = 0.0

	args := []string{"-a", "-X", "--format=Allocgres", "--state=RUNNING", "--noheader", "--parsable2"}
	output := string(Execute("sacct", args))
	if len(output) > 0 {
		for _, line := range strings.Split(output, "\n") {
			if len(line) > 0 {
				line = strings.Trim(line, "\"")
				descriptor := strings.TrimPrefix(line, "gpu:")
				job_gpus, _ := strconv.ParseFloat(descriptor, 64)
				num_gpus += job_gpus
			}
		}
	}

	return num_gpus
}
*/

func ParseAllocatedGPUs(data []byte) float64 {
	var num_gpus = 0.0
	var sinfoResp SinfoResponse

	if err := json.Unmarshal(data, &sinfoResp); err != nil {
		log.Errorf("Failed to parse JSON: %v", err)
		return 0.0
	}

	for _, node := range sinfoResp.Sinfo {
		if node.Gres.Used != "" && strings.Contains(node.Gres.Used, "gpu:") {
			// Parse GPU usage like "gpu:2" or "gpu:A30:4" etc.
			re := regexp.MustCompile(`gpu:(?:[^:]*:)?(\d+)`)
			matches := re.FindAllStringSubmatch(node.Gres.Used, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					gpu_count, err := strconv.ParseFloat(match[1], 64)
					if err == nil {
						num_gpus += gpu_count
					}
				}
			}
		}
	}

	return num_gpus
}

func ParseTotalGPUs() float64 {
	var num_gpus = 0.0

	args := []string{"--json"}
	output := Execute("sinfo", args)

	var sinfoResp SinfoResponse
	if err := json.Unmarshal(output, &sinfoResp); err != nil {
		log.Errorf("Failed to parse JSON: %v", err)
		return 0.0
	}

	for _, node := range sinfoResp.Sinfo {
		if node.Gres.Total != "" && strings.Contains(node.Gres.Total, "gpu:") {
			// Parse GPU total like "gpu:A100:8" or "gpu:TITAN:1" etc.
			re := regexp.MustCompile(`gpu:(?:[^:]*:)?(\d+)`)
			matches := re.FindAllStringSubmatch(node.Gres.Total, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					gpu_count, err := strconv.ParseFloat(match[1], 64)
					if err == nil {
						num_gpus += gpu_count
					}
				}
			}
		}
	}

	return num_gpus
}

func ParseGPUsMetrics() *GPUsMetrics {
	var gm GPUsMetrics
	total_gpus := ParseTotalGPUs()

	// Get allocated GPUs data using JSON
	args := []string{"-a", "--json", "--state=allocated"}
	allocated_data := Execute("sinfo", args)
	allocated_gpus := ParseAllocatedGPUs(allocated_data)

	gm.alloc = allocated_gpus
	gm.idle = total_gpus - allocated_gpus
	gm.total = total_gpus
	if total_gpus > 0 {
		gm.utilization = allocated_gpus / total_gpus
	} else {
		gm.utilization = 0
	}
	return &gm
}

// Execute the sinfo command and return its output
func Execute(command string, arguments []string) []byte {
	cmd := exec.Command(command, arguments...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	out, _ := ioutil.ReadAll(stdout)
	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
	return out
}

/*
 * Implement the Prometheus Collector interface and feed the
 * Slurm scheduler metrics into it.
 * https://godoc.org/github.com/prometheus/client_golang/prometheus#Collector
 */

func NewGPUsCollector() *GPUsCollector {
	return &GPUsCollector{
		alloc:       prometheus.NewDesc("slurm_gpus_alloc", "Allocated GPUs", nil, nil),
		idle:        prometheus.NewDesc("slurm_gpus_idle", "Idle GPUs", nil, nil),
		total:       prometheus.NewDesc("slurm_gpus_total", "Total GPUs", nil, nil),
		utilization: prometheus.NewDesc("slurm_gpus_utilization", "Total GPU utilization", nil, nil),
	}
}

type GPUsCollector struct {
	alloc       *prometheus.Desc
	idle        *prometheus.Desc
	total       *prometheus.Desc
	utilization *prometheus.Desc
}

// Send all metric descriptions
func (cc *GPUsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cc.alloc
	ch <- cc.idle
	ch <- cc.total
	ch <- cc.utilization
}
func (cc *GPUsCollector) Collect(ch chan<- prometheus.Metric) {
	cm := GPUsGetMetrics()
	ch <- prometheus.MustNewConstMetric(cc.alloc, prometheus.GaugeValue, cm.alloc)
	ch <- prometheus.MustNewConstMetric(cc.idle, prometheus.GaugeValue, cm.idle)
	ch <- prometheus.MustNewConstMetric(cc.total, prometheus.GaugeValue, cm.total)
	ch <- prometheus.MustNewConstMetric(cc.utilization, prometheus.GaugeValue, cm.utilization)
}
