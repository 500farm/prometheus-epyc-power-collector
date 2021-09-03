package main

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const AMD_MSR_PWR_UNIT = 0xC0010299
const AMD_MSR_CORE_ENERGY = 0xC001029A
const AMD_MSR_PACKAGE_ENERGY = 0xC001029B

const AMD_ENERGY_UNIT_MASK = 0x1F00
const AMD_ENERGY_VALUE_MASK = 0xFFFF

const MAX_CORES = 1024

func readMsr(msr *os.File, offset int64) uint64 {
	buf := make([]byte, 8)
	_, err := msr.ReadAt(buf, offset)
	if err != nil {
		log.Fatal(err)
	}
	return binary.LittleEndian.Uint64(buf)
}

func main() {
	outputDir := os.Getenv("TEMP") + "/prometheus"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal(err)
	}
	outputFile := outputDir + "/epyc_power_collector.prom"

	_, err := exec.Command("/usr/sbin/modprobe", "msr").Output()
	if err != nil {
		log.Fatal(err)
	}

	coreToPackageMap := make(map[int]int)
	coreMsrs := make(map[int]*os.File)

	for i := 0; i < MAX_CORES; i++ {
		t, err := ioutil.ReadFile("/sys/devices/system/cpu/cpu" + strconv.Itoa(i) + "/topology/physical_package_id")
		if err != nil {
			break
		}
		coreToPackageMap[i], _ = strconv.Atoi(strings.TrimSpace(string(t)))

		fd, err := os.OpenFile("/dev/cpu/"+strconv.Itoa(i)+"/msr", os.O_RDONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}
		coreMsrs[i] = fd
	}

	energyUnit := math.Pow(0.5, float64(
		(readMsr(coreMsrs[0], AMD_MSR_PWR_UNIT)&AMD_ENERGY_UNIT_MASK)>>8,
	))

	for {
		coreEnergy := make(map[int]float64)
		packageEnergy := make(map[int]float64)

		start := time.Now()

		for i, msr := range coreMsrs {
			pkg := coreToPackageMap[i]
			coreEnergy[i] = float64(readMsr(msr, AMD_MSR_CORE_ENERGY) & AMD_ENERGY_VALUE_MASK)
			packageEnergy[pkg] = float64(readMsr(msr, AMD_MSR_PACKAGE_ENERGY) & AMD_ENERGY_VALUE_MASK)
		}

		time.Sleep(5 * time.Second)
		dt := time.Now().Sub(start).Seconds()

		rollover := false
		for i, msr := range coreMsrs {
			pkg := coreToPackageMap[i]
			coreEnergy[i] -= float64(readMsr(msr, AMD_MSR_CORE_ENERGY) & AMD_ENERGY_VALUE_MASK)
			packageEnergy[pkg] -= float64(readMsr(msr, AMD_MSR_PACKAGE_ENERGY) & AMD_ENERGY_VALUE_MASK)
			if coreEnergy[i] > 0 || packageEnergy[pkg] > 0 {
				// rollover - consider data invalid
				rollover = true
			}
		}

		if !rollover {
			output := "# HELP node_cpu_power_cores_watts Average power consumption by all cores of this CPU\n" +
				"# TYPE node_cpu_power_cores_watts gauge\n" +
				"# HELP node_cpu_power_package_watts Average power consumption by this CPU\n" +
				"# TYPE node_cpu_power_package_watts gauge\n"

			for pkg, w := range packageEnergy {
				output += fmt.Sprintf("node_cpu_power_package_watts{package=\"%d\"} %f\n", pkg, -w*energyUnit/dt)
				w1 := 0.0
				for core, w2 := range coreEnergy {
					if coreToPackageMap[core] == pkg {
						w1 += w2
					}
				}
				output += fmt.Sprintf("node_cpu_power_cores_watts{package=\"%d\"} %f\n", pkg, -w1*energyUnit/dt)
			}

			ioutil.WriteFile(outputFile, []byte(output), 0644)
		}
	}
}
