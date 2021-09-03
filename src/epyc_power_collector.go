package main

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

const AMD_MSR_PWR_UNIT = 0xC0010299
const AMD_MSR_CORE_ENERGY = 0xC001029A
const AMD_MSR_PACKAGE_ENERGY = 0xC001029B

const AMD_ENERGY_UNIT_MASK = 0x1F00

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
	coreToPackageMap := make(map[int]int)
	coreMsrs := make(map[int]*os.File)

	for i := 0; i < MAX_CORES; i++ {
		if i%2 == 1 {
			// skip SMT threads
			continue
		}

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

	energy_unit := math.Pow(0.5, float64(
		(readMsr(coreMsrs[0], AMD_MSR_PWR_UNIT)&AMD_ENERGY_UNIT_MASK)>>8,
	))

	for {
		packageCoresTotalEnergy := make(map[int]float64)
		packageTotalEnergy := make(map[int]float64)

		start := time.Now()

		for i, msr := range coreMsrs {
			pkg := coreToPackageMap[i]
			packageCoresTotalEnergy[pkg] += float64(readMsr(msr, AMD_MSR_CORE_ENERGY))
			packageTotalEnergy[pkg] += float64(readMsr(msr, AMD_MSR_PACKAGE_ENERGY))
		}

		time.Sleep(5 * time.Second)
		dt := time.Now().Sub(start).Seconds()

		for i, msr := range coreMsrs {
			pkg := coreToPackageMap[i]
			packageCoresTotalEnergy[pkg] -= float64(readMsr(msr, AMD_MSR_CORE_ENERGY))
			packageTotalEnergy[pkg] -= float64(readMsr(msr, AMD_MSR_PACKAGE_ENERGY))
		}

		output := "# HELP cpu_power_cores_watts Average power consumption by all cores of this CPU\n" +
			"# TYPE cpu_power_cores_watts gauge\n" +
			"# HELP cpu_power_package_watts Average power consumption by this CPU\n" +
			"# TYPE cpu_power_package_watts gauge\n"

		for pkg, w := range packageCoresTotalEnergy {
			w1 := packageTotalEnergy[pkg] / float64(len(coreMsrs))
			output += fmt.Sprintf("cpu_power_cores_watts{package=\"%d\"} %f\n", pkg, -w*energy_unit*dt)
			output += fmt.Sprintf("cpu_power_package_watts{package=\"%d\"} %f\n", pkg, -w1*energy_unit*dt)
		}

		ioutil.WriteFile("/tmp/prometheus/epyc_power_collector.prom", []byte(output), 0644)
	}
}
