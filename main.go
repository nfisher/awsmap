package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/awslabs/aws-sdk-go/service/elb"
)

type Config struct {
	Region        string
	InstanceCount int64
	IsDownload    bool
	IsServe       bool
	Filename      string
}

func main() {
	var config *Config = new(Config)

	flag.BoolVar(&config.IsServe, "serve", false, "Start server.")
	flag.BoolVar(&config.IsDownload, "download", false, "Retrieve latest data.")
	flag.Int64Var(&config.InstanceCount, "instances", 100, "Number of running instances.")
	flag.StringVar(&config.Region, "region", "eu-west-1", "AWS region to map.")
	flag.StringVar(&config.Filename, "filename", "region.json", "Storage location of JSON files.")

	flag.Parse()

	var region *AwsRegion
	var err error

	if config.IsDownload {
		region, err = fetchRegion(config)
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

		graph := buildGraph(config, region)
		handler := &GraphHandler{
			graph,
			config,
		}

		server := &http.Server{
			Addr:    "127.0.0.1:8080",
			Handler: handler,
		}

		log.Fatal(server.ListenAndServe())
	}
}

type GraphHandler struct {
	*Graph
	*Config
}

type Dendogram struct {
	Name     string       `json:"name"`
	Children []*Dendogram `json:"children,omitempty"`
}

func (gs *GraphHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/" {
		fmt.Fprintf(w, IndexPage)
		return
	}

	if req.URL.Path == "/region.json" {
		root, err := generateDendogram(gs.Config, gs.Graph)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		enc := json.NewEncoder(w)

		err = enc.Encode(root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		return
	}

	http.NotFound(w, req)
}

func IsFromAz(az string) RelationshipFilterFunc {
	return func(e *Edge) bool {
		return az == e.From.Id
	}
}

func IsFromSubnet(subnet string) RelationshipFilterFunc {
	return func(e *Edge) bool {
		sn, ok := e.From.Value.(*ec2.Subnet)
		if !ok {
			return false
		}

		return subnet == *sn.SubnetID
	}
}

func IsToA(t Type) RelationshipFilterFunc {
	return func(e *Edge) bool {
		return e.To.Type == t
	}
}

func IsToSubnetInVpc(vpc string) RelationshipFilterFunc {
	return func(e *Edge) bool {
		sn, ok := e.To.Value.(*ec2.Subnet)
		if !ok {
			return false
		}

		return vpc == *sn.VPCID
	}
}

func generateDendogram(config *Config, graph *Graph) (root *Dendogram, err error) {
	root = &Dendogram{
		Name: config.Region,
	}

	azs, err := graph.GetNeighbours(config.Region)
	if err != nil {
		return nil, err
	}

	instanceSeen := make(map[string]bool)

	for _, relationship := range azs {
		if relationship.Relationship == "houses" {
			az := &Dendogram{Name: relationship.To.Id}
			root.Children = append(root.Children, az)

			for _, vpc := range azs {
				if vpc.Relationship == "hosts" {
					vpcNode := &Dendogram{Name: vpc.To.Id}
					az.Children = append(az.Children, vpcNode)
					for _, n := range graph.GetNeighboursBy(IsToSubnetInVpc(vpcNode.Name), IsFromAz(az.Name)) {
						subnet := &Dendogram{Name: n.To.Id}
						name := ""
						sn := n.To.Value.(*ec2.Subnet)
						for _, tag := range sn.Tags {
							if *tag.Key == "Name" {
								name = *tag.Value
							}
						}

						subnet.Name = subnet.Name + " " + name

						vpcNode.Children = append(vpcNode.Children, subnet)
						for _, elbs := range graph.GetNeighboursBy(IsFromSubnet(n.To.Id), IsToA(LoadBalancer)) {
							elbDendogram := &Dendogram{Name: elbs.To.Id}
							subnet.Children = append(subnet.Children, elbDendogram)

							elbDesc, ok := elbs.To.Value.(*elb.LoadBalancerDescription)
							if !ok {
								log.Println("These are not the LBs you're looking for!")
								break
							}

							for _, elbInstance := range elbDesc.Instances {
								instanceId := *elbInstance.InstanceID
								i := &Dendogram{Name: instanceId}
								elbDendogram.Children = append(elbDendogram.Children, i)
								instanceSeen[instanceId] = true
							}
						}

						for _, instanceRel := range graph.GetNeighboursBy(IsFromSubnet(n.To.Id), IsToA(Instance)) {
							i := &Dendogram{Name: instanceRel.To.Id}
							inst := instanceRel.To.Value.(*ec2.Instance)
							name := ""
							for _, tag := range inst.Tags {
								if *tag.Key == "Name" {
									name = *tag.Value
								}
							}

							if instanceSeen[instanceRel.To.Id] {
								i.Name = "<<" + i.Name + ">>"
							}

							i.Name = name + " " + i.Name

							subnet.Children = append(subnet.Children, i)
						}
					}
				}
			}
		}
	}

	return root, nil
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

		azNode, err := graph.GetNode(*net.AvailabilityZone)
		if err == NodeNotFound {
			azNode = graph.AddNode(*net.AvailabilityZone, AvailabilityZone, *net.AvailabilityZone)
			graph.AddNeighbour(regionNode, "houses", azNode)
			graph.AddNeighbour(azNode, "housed_by", regionNode)
		}

		vpcNode, err := graph.GetNode(*net.VPCID)
		if err != nil {
			log.Printf("subnet[%v] not associated with a known vpc[%v].\n", *net.SubnetID, *net.VPCID)
			continue
		}
		graph.AddNeighbour(vpcNode, "allocates_network", subnetNode)
		graph.AddNeighbour(subnetNode, "network_allocated_by", vpcNode)
		graph.AddNeighbour(azNode, "hosts_network", subnetNode)
		graph.AddNeighbour(subnetNode, "network_hosted_by", azNode)
	}

	// add instances
	for _, i := range region.Instances {
		instanceNode := graph.AddNode(*i.InstanceID, Instance, i)
		subnetNode, err := graph.GetNode(*i.SubnetID)
		if err != nil {
			log.Printf("instance[%v] not associated with a known subnet[%v].", *i.InstanceID, *i.SubnetID)
		}

		graph.AddNeighbour(subnetNode, "allocates_ip", instanceNode)
		graph.AddNeighbour(instanceNode, "ip_allocated_from", subnetNode)

	}

	// add elbs
	for _, elb := range region.LoadBalancers {
		elbNode := graph.AddNode(*elb.LoadBalancerName, LoadBalancer, elb)
		for _, subnetId := range elb.Subnets {
			subnetNode, err := graph.GetNode(*subnetId)
			if err != nil {
				continue
			}
			graph.AddNeighbour(subnetNode, "homes", elbNode)
			graph.AddNeighbour(elbNode, "homed_in", subnetNode)
		}

		for _, instance := range elb.Instances {
			instanceNode, err := graph.GetNode(*instance.InstanceID)
			if err != nil {
				continue
			}
			graph.AddNeighbour(elbNode, "proxies", instanceNode)
			graph.AddNeighbour(instanceNode, "proxied_by", elbNode)
		}
	}

	// add SGs, IGW, ACLS, Routes

	return graph
}

const IndexPage = `<!DOCTYPE html>
<meta charset="utf-8">
<style>

.node circle {
  fill: #fff;
  stroke: steelblue;
  stroke-width: 1.5px;
}

.node {
  font: 10px sans-serif;
}

.link {
  fill: none;
  stroke: #ccc;
  stroke-width: 1.5px;
}

</style>
<body>
<script src="http://d3js.org/d3.v3.min.js"></script>
<script>

var width = 960,
    height = 960;

var cluster = d3.layout.cluster()
    .sort(d3.descending)
    .size([height, width - 260]);

var diagonal = d3.svg.diagonal()
    .projection(function(d) { return [d.y, d.x]; });

var svg = d3.select("body").append("svg")
    .attr("width", width)
    .attr("height", height)
  .append("g")
    .attr("transform", "translate(55,0)");

d3.json("/region.json", function(error, root) {
  var nodes = cluster.nodes(root),
      links = cluster.links(nodes);

  var link = svg.selectAll(".link")
      .data(links)
    .enter().append("path")
      .attr("class", "link")
      .attr("d", diagonal);

  var node = svg.selectAll(".node")
      .data(nodes)
    .enter().append("g")
      .attr("class", "node")
      .attr("transform", function(d) { return "translate(" + d.y + "," + d.x + ")"; })

  node.append("circle")
      .attr("r", 4.5);

  node.append("text")
      .attr("dx", function(d) { return d.children ? -8 : 8; })
      .attr("dy", 3)
      .style("text-anchor", function(d) { return d.children ? "end" : "start"; })
      .text(function(d) { return d.name; });
});

d3.select(self.frameElement).style("height", height + "px");

</script>`
