package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
)

type Config struct {
	Region        string
	InstanceCount int64
	IsDownload    bool
	IsServe       bool
	Filename      string
}

func main() {
	var config Config

	flag.BoolVar(&config.IsServe, "serve", false, "Start server.")
	flag.BoolVar(&config.IsDownload, "download", false, "Retrieve latest data.")
	flag.Int64Var(&config.InstanceCount, "instances", 100, "Number of running instances.")
	flag.StringVar(&config.Region, "region", "eu-west-1", "AWS region to map.")
	flag.StringVar(&config.Filename, "filename", "region.json", "Storage location of JSON files.")

	flag.Parse()

	var region *AwsRegion
	var err error

	if config.IsDownload {
		region, err = fetchRegion(&config)
		if err != nil {
			log.Fatal(err)
		}

		f, err := os.OpenFile(config.Filename, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}

		enc := json.NewEncoder(f)
		err = enc.Encode(region)
		if err != nil {
			log.Fatal(err)
		}
	}

	if config.IsServe {
		f, err := os.Open(config.Filename)
		if err != nil {
			log.Fatal(err)
		}

		dec := json.NewDecoder(f)
		err = dec.Decode(&region)
		if err != nil {
			log.Fatal(err)
		}

		buildGraph(&config, region)
	}
}

const (
	Region Type = iota
	Vpc
	Subnet
	AvailabilityZone
	RouteTable
	InternetGateway
	Instance
	LoadBalancer
	Acl
)

func buildGraph(config *Config, region *AwsRegion) (graph *Graph) {
	graph = NewGraph()
	// add region as root
	regionNode := graph.AddNode(config.Region, Region, region)

	// add VPCs
	for _, vpc := range region.Vpcs {
		vpcNode := graph.AddNode(*vpc.VPCID, Vpc, vpc)
		graph.AddNeighbour(regionNode, "hosts", vpcNode)
		graph.AddNeighbour(vpcNode, "hosted_by", regionNode)
	}

	// add subnets and AZs
	for _, net := range region.Subnets {
		subnetNode := graph.AddNode(*net.SubnetID, Subnet, net)

		azNode := graph.AddNode(*net.AvailabilityZone, AvailabilityZone, *net.AvailabilityZone)
		graph.AddNeighbour(regionNode, "houses", azNode)
		graph.AddNeighbour(azNode, "housed_by", regionNode)

		vpcNode, err = graph.GetNode(*net.VPCID)
		if err != nil {
			log.Printf("subnet[%v] not associated with a known vpc[%v].\n", *net.SubnetID, *net.VPCID)
			continue
		}
		graph.AddNeighbour(vpcNode, "allocates_network", subnetNode)
		graph.AddNeighbour(subnetNode, "network_allocated_by", vpcNode)
	}

	// add instances
	for _, i := range region.Instances {
		instanceNode := graph.AddNode(*i.InstanceID, Instance, i)
		subnetNode, err = graph.GetNode(*i.SubnetID)
		if err != nil {
			log.Printf("instance[%v] not associated with a known subnet[%v].", *i.InstanceID, *i.SubnetID)
		}

		graph.AddNeighbour(subnetNode, "allocates_ip", instanceNode)
		graph.AddNeighbour(instanceNode, "ip_allocated_from", subnetNode)

	}

	// add elbs
	for _, elb := range region.LoadBalancers {
		elbNode := graph.AddNode(*elb.LoadBalancerName, LoadBalancer, elb)
		subnetNode, err := graph.GetNode(*elb.SubnetID)

		graph.AddNeighbour(subnetNode, "homes", elbNode)
		graph.AddNeighbour(elbNode, "homed_in", subnetNode)
	}

	// add SGs, IGW, ACLS, Routes

	return graph
}
