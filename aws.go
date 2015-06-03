package main

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/aws/awsutil"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/awslabs/aws-sdk-go/service/elb"
)

func writeSecGroups(resp *ec2.DescribeSecurityGroupsOutput, w io.Writer) (err error) {
	_, err = fmt.Fprintln(w, "---")
	if err != nil {
		return err
	}

	for _, sg := range resp.SecurityGroups {
		fmt.Fprintln(w, *sg.GroupID+":")
		fmt.Fprintln(w, "    name: "+awsutil.StringValue(sg.GroupName))
		fmt.Fprintln(w, "    rules:")
		for _, rule := range sg.IPPermissions {
			ips := make([]string, 0, 16)
			for _, ip := range rule.IPRanges {
				ips = append(ips, awsutil.StringValue(ip.CIDRIP))
			}

			ugs := make([]string, 0, 16)
			for _, ug := range rule.UserIDGroupPairs {
				ugs = append(ugs, awsutil.StringValue(ug.GroupID))
			}

			if rule.FromPort != nil {
				fmt.Fprintln(w, "        - from: "+awsutil.StringValue(rule.FromPort))
				fmt.Fprintln(w, "          to: "+awsutil.StringValue(rule.ToPort))
				fmt.Fprintln(w, "          cidr: ["+strings.Join(ips, ",")+"]")
				fmt.Fprintln(w, "          groups: ["+strings.Join(ugs, ",")+"]")
			} else { // icmp rule
				fmt.Fprintln(w, "        - cidr: ["+strings.Join(ips, ",")+"]")
				fmt.Fprintln(w, "          groups: ["+strings.Join(ugs, ",")+"]")
			}
		}
	}

	return nil
}

func writeElbs(resp *elb.DescribeLoadBalancersOutput, w io.Writer) (err error) {
	_, err = fmt.Fprintln(w, "---")
	if err != nil {
		return err
	}

	for _, elb := range resp.LoadBalancerDescriptions {
		instances := make([]string, 0, 16)
		for _, i := range elb.Instances {
			instances = append(instances, awsutil.StringValue(i.InstanceID))
		}

		subnets := make([]string, 0, 16)
		for _, s := range elb.Subnets {
			subnets = append(subnets, awsutil.StringValue(s))
		}

		fmt.Fprintln(w, *elb.LoadBalancerName+":")
		fmt.Fprintln(w, "    dns: "+awsutil.StringValue(elb.DNSName))
		fmt.Fprintln(w, "    instances: ["+strings.Join(instances, ",")+"]")
		fmt.Fprintln(w, "    subnets: ["+strings.Join(subnets, ",")+"]")
	}

	return nil
}

func writeInstances(resp *ec2.DescribeInstancesOutput, w io.Writer) (err error) {
	_, err = fmt.Fprintln(w, "---")
	if err != nil {
		return err
	}

	for _, r := range resp.Reservations {
		for _, i := range r.Instances {
			var name string
			for _, tag := range i.Tags {
				if *tag.Key == "Name" {
					name = awsutil.StringValue(tag.Value)
				}
			}

			sgs := make([]string, 0, 16)
			for _, sg := range i.SecurityGroups {
				sgs = append(sgs, awsutil.StringValue(sg.GroupID))
			}
			fmt.Fprintln(w, *i.InstanceID+":")
			fmt.Fprintln(w, "    az: "+awsutil.StringValue(i.Placement.AvailabilityZone))
			fmt.Fprintln(w, "    name: "+name)
			fmt.Fprintln(w, "    sgs: "+strings.Join(sgs, ","))
			fmt.Fprintln(w, "    subnet: "+awsutil.StringValue(i.SubnetID))
		}
	}

	return nil
}

func fetchVpcs(cfg *aws.Config, config *Config, region *AwsRegion) (err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeVPCsInput{}

	resp, err := svc.DescribeVPCs(params)
	if err != nil {
		return err
	}

	region.Vpcs = resp.VPCs

	return nil
}

func fetchSubnets(cfg *aws.Config, config *Config, region *AwsRegion) (err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeSubnetsInput{}

	resp, err := svc.DescribeSubnets(params)
	if err != nil {
		return err
	}

	region.Subnets = resp.Subnets

	return nil
}

func fetchInstances(cfg *aws.Config, runtimeConfig *Config, region *AwsRegion) (err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeInstancesInput{
		DryRun:     aws.Boolean(false),
		MaxResults: aws.Long(runtimeConfig.InstanceCount),
		Filters: []*ec2.Filter{
			&ec2.Filter{ // Required
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
				},
			},
		},
	}

	resp, err := svc.DescribeInstances(params)
	if err != nil {
		return err
	}

	for _, reservation := range resp.Reservations {
		region.Instances = append(region.Instances, reservation.Instances...)
	}

	return nil
}

func fetchElbs(cfg *aws.Config, config *Config, region *AwsRegion) (err error) {
	svc := elb.New(cfg)

	params := &elb.DescribeLoadBalancersInput{}

	resp, err := svc.DescribeLoadBalancers(params)
	if err != nil {
		return err
	}

	region.LoadBalancers = resp.LoadBalancerDescriptions

	return nil
}

func fetchSecurityGroups(cfg *aws.Config, config *Config, region *AwsRegion) (err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeSecurityGroupsInput{}

	resp, err := svc.DescribeSecurityGroups(params)
	if err != nil {
		return err
	}

	region.SecurityGroups = resp.SecurityGroups

	return nil
}

func fetchAcls(cfg *aws.Config, config *Config, region *AwsRegion) (err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeNetworkACLsInput{}

	resp, err := svc.DescribeNetworkACLs(params)
	if err != nil {
		return err
	}

	region.Acls = resp.NetworkACLs

	return nil
}

func fetchRoutes(cfg *aws.Config, config *Config, region *AwsRegion) (err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeRouteTablesInput{}

	resp, err := svc.DescribeRouteTables(params)
	if err != nil {
		return err
	}

	region.Routes = resp.RouteTables

	return nil
}

func fetchGateways(cfg *aws.Config, config *Config, region *AwsRegion) (err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeInternetGatewaysInput{}

	resp, err := svc.DescribeInternetGateways(params)
	if err != nil {
		return err
	}

	region.Gateways = resp.InternetGateways

	return nil
}

type AwsRegion struct {
	Acls           []*ec2.NetworkACL
	Gateways       []*ec2.InternetGateway
	Instances      []*ec2.Instance
	LoadBalancers  []*elb.LoadBalancerDescription
	Routes         []*ec2.RouteTable
	SecurityGroups []*ec2.SecurityGroup
	Subnets        []*ec2.Subnet
	Vpcs           []*ec2.VPC
}

type MultiError []error

func (me MultiError) Error() string {
	s := ""
	for _, v := range me {
		s += v.Error() + "\n"
	}

	return s
}

func (me MultiError) HasError() bool {
	for _, v := range me {
		if v != nil {
			return true
		}
	}
	return false
}

type callable func(cfg *aws.Config, config *Config, region *AwsRegion) error

func fetchRegion(config *Config) (region *AwsRegion, err error) {
	cfg := &aws.Config{Region: config.Region}
	var wg sync.WaitGroup
	region = &AwsRegion{}

	fns := []callable{fetchInstances, fetchSecurityGroups, fetchSubnets, fetchElbs, fetchVpcs, fetchAcls, fetchRoutes, fetchGateways}
	errors := make(MultiError, len(fns), len(fns))

	for i, fn := range fns {
		wg.Add(1)
		go func(fn callable, i int) {
			err = fn(cfg, config, region)
			errors[i] = err
			wg.Done()
		}(fn, i)
	}

	wg.Wait()

	if errors.HasError() {
		return nil, errors
	}

	return region, nil
}
