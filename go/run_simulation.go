package main

import (
	"fmt"
	"os"
	"os/exec"
)

func runSimulation(command string, logfile string) error {
	logFile, err := os.Create(logfile)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command("bash", "-c", command)

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	fmt.Println("Starting simulation...")
	fmt.Println(command)

	err = cmd.Run()
	if err != nil {
		return err
	}

	fmt.Println("Simulation finished.")
	return nil
}

func main() {

	command := `
./leodust \
--simulationConfig ./resources/configs/simulationManualConfig-3000.yaml \
--islConfig ./resources/configs/islMstConfig.yaml \
--groundLinkConfig ./resources/configs/groundLinkNearestConfig.yaml \
--computingConfig ./resources/configs/computingConfig.yaml \
--routerConfig ./resources/configs/routerAStarConfig.yaml \
--simulationStateOutputFile ./precomputed/mst/precomputed-3000.gob \
--simulationPlugins DummyPlugin \
--statePlugins DummyPlugin
`

	logfile := "./results/mst/simulated/3000.log"

	err := runSimulation(command, logfile)
	if err != nil {
		fmt.Println("Simulation failed:", err)
		os.Exit(1)
	}

	fmt.Println("Log written to:", logfile)
}
