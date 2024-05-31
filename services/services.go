package services

import (
	"CTCollector/datastruct"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type Collector struct {
	RunStats         []datastruct.TestRun
	RunTasks         []datastruct.RunTask
	CurrentTask      int
	RunningInstances []string
	KeyName          string
	mu               sync.Mutex
}

func awsCLI(args ...string) (string, error) {
	cmd := exec.Command("aws", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("AWS CLI Error:", stderr.String())
		return "", fmt.Errorf("%s: %w", stderr.String(), err)
	}
	return out.String(), nil
}

type RunInstancesOutput struct {
	Instances []struct {
		InstanceId string `json:"InstanceId"`
	} `json:"Instances"`
}

func extractInstanceIDsFromJSON(jsonData string) ([]string, error) {
	var result RunInstancesOutput
	err := json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %v", err)
	}
	var ids []string
	for _, inst := range result.Instances {
		ids = append(ids, inst.InstanceId)
	}
	return ids, nil
}

func SpawnClients(collector *Collector, client_count string, server_ip string, collector_ip string, reveal int) error {
	region := "us-east-1"
	instanceType := "t2.micro"
	securityGroupID := "sg-03c26d167c72f8254"
	count := client_count

	// Get the latest Amazon Linux 2 AMI ID
	amiID, err := awsCLI("ec2", "describe-images", "--owners", "amazon",
		"--filters", "Name=name,Values=amzn2-ami-hvm-*-x86_64-gp2",
		"Name=state,Values=available",
		"--query", "Images | sort_by(@, &CreationDate) | [-1].ImageId",
		"--output", "text", "--region", region)
	if err != nil {
		fmt.Println("Error getting AMI ID:", err)
		return err
	}
	amiID = strings.TrimSpace(amiID)
	fmt.Println("Using AMI ID:", amiID)

	// Find the default subnet in the first available zone
	subnetID, err := awsCLI("ec2", "describe-subnets", "--filters", "Name=default-for-az,Values=true", "--query", "Subnets[0].SubnetId", "--output", "text", "--region", region)
	if err != nil {
		fmt.Println("Error getting subnet ID:", err)
		return err
	}

	subnetID = strings.TrimSpace(subnetID)
	fmt.Println("Using default subnet ID:", subnetID)

	client_script_user_data := fmt.Sprintf(`#!/bin/bash
	sudo su
	cd ~
	yum install go -y
	yum install git -y
	git clone https://github.com/Andyluchina/CTClient
	cd CTClient
	go build main.go
	./main %s %s %s`, server_ip, strconv.Itoa(reveal), collector_ip)

	userDataEncoded := base64.StdEncoding.EncodeToString([]byte(client_script_user_data))
	// Start EC2 instances
	fmt.Println("Launching instances...")
	launchOutput, err := awsCLI("ec2", "run-instances", "--image-id", amiID, "--instance-type", instanceType, "--count", count, "--key-name", collector.KeyName, "--security-group-ids", securityGroupID, "--subnet-id", subnetID, "--user-data", userDataEncoded, "--region", region)
	if err != nil {
		fmt.Println("Error launching instances:", err)
		return err
	}
	fmt.Println("Client Instances launched.")

	// Extract instance IDs (assume jq is installed or use another method to parse JSON)
	instanceIDs, err := extractInstanceIDsFromJSON(launchOutput)
	if err != nil {
		fmt.Println("Error extracting instance IDs:", err)
		return err
	}

	collector.RunningInstances = append(collector.RunningInstances, instanceIDs...)
	return nil
}

func Cleanup(collector *Collector) error {
	// Terminate instances
	fmt.Println("Terminating instances...")
	// Assuming instanceIDs is of type []string and already populated
	err := terminateInstances(collector.RunningInstances)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Instances terminated.")

	// Delete the key pair
	fmt.Println("Deleting key pair...")
	if _, err := awsCLI("ec2", "delete-key-pair", "--key-name", collector.KeyName, "--region", "us-east-1"); err != nil {
		fmt.Println("Error deleting key pair:", err)
		return err
	}
	os.Remove(collector.KeyName + ".pem")
	fmt.Println("Key pair deleted.")

	fmt.Println("Script completed.")
	return nil
}

func terminateInstances(instanceIDs []string) error {
	args := []string{"ec2", "terminate-instances", "--instance-ids"}
	// Append each instance ID as a separate element in the slice
	args = append(args, instanceIDs...)
	args = append(args, "--region", "us-east-1") // Specify the region if needed

	output, err := awsCLI(args...)
	if err != nil {
		return fmt.Errorf("error terminating instances: %v, output: %s", err, output)
	}
	fmt.Println("Terminate Instances Output:", output)
	return nil
}

func (collector *Collector) ReportStatsClient(req *datastruct.ClientStats, reply *datastruct.ReportStatsReply) error {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	fmt.Println("Client Report Received")
	collector.RunStats[collector.CurrentTask].Clients = append(collector.RunStats[collector.CurrentTask].Clients, *req)
	reply.Status = true
	return nil
}

func (collector *Collector) ReportStatsAuditor(req *datastruct.AuditorReport, reply *datastruct.ReportStatsReply) error {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	fmt.Println("Auditor Report Received")
	collector.RunStats[collector.CurrentTask].Auditor = *req
	reply.Status = true

	// write collected data to a file
	WriteRevealInfoToDatabase(collector.RunStats)
	Cleanup(collector)
	return nil
}

func WriteRevealInfoToDatabase(db []datastruct.TestRun) error {
	// Marshal the updated array back to a byte slice
	updatedData, err := json.Marshal(db)
	// fmt.Println(updatedData)
	if err != nil {
		return err
	}

	// Write the updated data to the file
	err = os.WriteFile("report.json", updatedData, 0644)
	if err != nil {
		return err
	}

	return nil
}
