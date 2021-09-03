package main

import (
	"encoding/binary"
	"io/ioutil"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

const AMD_MSR_PWR_UNIT = 0xC0010299
const AMD_MSR_CORE_ENERGY = 0xC001029A
const AMD_MSR_PACKAGE_ENERGY = 0xC001029B

const AMD_TIME_UNIT_MASK = 0xF0000
const AMD_ENERGY_UNIT_MASK = 0x1F00
const AMD_POWER_UNIT_MASK = 0xF

const MAX_CORES = 1024

func readMsr(msr []byte, offset int) uint64 {
	return binary.LittleEndian.Uint64(msr[offset : offset+8])
}

func main() {
	coreToPackageMap := make(map[int]int)
	coreMsrs := make(map[int][]byte)

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
		t, err = ioutil.ReadFile("/dev/cpu/" + strconv.Itoa(i) + "/msr")
		if err != nil {
			log.Fatal(err)
		}
		coreMsrs[i] = t
	}

	log.Println(coreToPackageMap)
	log.Println(coreMsrs)

	msr := coreMsrs[0]
	core_energy_units := readMsr(msr, AMD_MSR_PWR_UNIT)
	log.Printf("Core energy units: %x\n", core_energy_units)

	time_unit := (core_energy_units & AMD_TIME_UNIT_MASK) >> 16
	energy_unit := (core_energy_units & AMD_ENERGY_UNIT_MASK) >> 8
	power_unit := core_energy_units & AMD_POWER_UNIT_MASK
	log.Printf("Time_unit: %d, Energy_unit: %d, Power_unit: %d\n", time_unit, energy_unit, power_unit)

	time_unit_d := math.Pow(0.5, float64(time_unit))
	energy_unit_d := math.Pow(0.5, float64(energy_unit))
	power_unit_d := math.Pow(0.5, float64(power_unit))
	log.Printf("Time_unit: %g, Energy_unit: %g, Power_unit: %g\n", time_unit_d, energy_unit_d, power_unit_d)

	packageCoresTotalEnergy := make(map[int]float64)
	packageTotalEnergy := make(map[int]float64)

	for i, msr := range coreMsrs {
		pkg := coreToPackageMap[i]
		packageCoresTotalEnergy[pkg] += float64(readMsr(msr, AMD_MSR_CORE_ENERGY)) * energy_unit_d
		packageTotalEnergy[pkg] += float64(readMsr(msr, AMD_MSR_PACKAGE_ENERGY)) * energy_unit_d
	}

	time.Sleep(100 * time.Millisecond)

	for i, msr := range coreMsrs {
		pkg := coreToPackageMap[i]
		packageCoresTotalEnergy[pkg] -= float64(readMsr(msr, AMD_MSR_CORE_ENERGY)) * energy_unit_d
		packageTotalEnergy[pkg] -= float64(readMsr(msr, AMD_MSR_PACKAGE_ENERGY)) * energy_unit_d
	}

	for pkg, w := range packageCoresTotalEnergy {
		log.Printf("Package %d cores W: %f\n", pkg, -w)
		log.Printf("Package %d total W: %f\n", pkg, -packageTotalEnergy[pkg]/float64(len(coreMsrs)))
	}
}
