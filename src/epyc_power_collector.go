package main

import (
	"encoding/binary"
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

	packageCoresTotalEnergy := make(map[int]float64)
	packageTotalEnergy := make(map[int]float64)

	for i, msr := range coreMsrs {
		pkg := coreToPackageMap[i]
		packageCoresTotalEnergy[pkg] += float64(readMsr(msr, AMD_MSR_CORE_ENERGY))
		packageTotalEnergy[pkg] += float64(readMsr(msr, AMD_MSR_PACKAGE_ENERGY))
	}

	time.Sleep(100 * time.Millisecond)

	for i, msr := range coreMsrs {
		pkg := coreToPackageMap[i]
		packageCoresTotalEnergy[pkg] -= float64(readMsr(msr, AMD_MSR_CORE_ENERGY))
		packageTotalEnergy[pkg] -= float64(readMsr(msr, AMD_MSR_PACKAGE_ENERGY))
	}

	for pkg, w := range packageCoresTotalEnergy {
		log.Printf("Package %d cores W: %f\n", pkg, -w*energy_unit)
		w = packageTotalEnergy[pkg] / float64(len(coreMsrs))
		log.Printf("Package %d total W: %f\n", pkg, -w*energy_unit)
	}
}
