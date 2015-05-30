package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/aws/awsutil"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/awslabs/aws-sdk-go/service/elb"
)

func getSubnets(cfg *aws.Config) (resp *ec2.DescribeSubnetsOutput, err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeSubnetsInput{}

	resp, err = svc.DescribeSubnets(params)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func writeJson(v interface{}, w io.Writer) (err error) {
	enc := json.NewEncoder(w)
	err = enc.Encode(v)
	return err
}

func getInstances(cfg *aws.Config, instanceCount int64) (resp *ec2.DescribeInstancesOutput, err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeInstancesInput{
		DryRun:     aws.Boolean(false),
		MaxResults: aws.Long(instanceCount),
		Filters: []*ec2.Filter{
			&ec2.Filter{ // Required
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
				},
			},
		},
	}

	resp, err = svc.DescribeInstances(params)
	if err != nil {
		return nil, err
	}

	return resp, nil
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

func getElbs(cfg *aws.Config) (resp *elb.DescribeLoadBalancersOutput, err error) {
	svc := elb.New(cfg)

	params := &elb.DescribeLoadBalancersInput{}

	resp, err = svc.DescribeLoadBalancers(params)
	if err != nil {
		return nil, err
	}

	return resp, nil
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

func getSecGroups(cfg *aws.Config) (resp *ec2.DescribeSecurityGroupsOutput, err error) {
	svc := ec2.New(cfg)

	params := &ec2.DescribeSecurityGroupsInput{}

	resp, err = svc.DescribeSecurityGroups(params)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

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

func online(region string, instanceCount int64) {
	cfg := &aws.Config{Region: region}
	var wg sync.WaitGroup

	var secGroups *ec2.DescribeSecurityGroupsOutput
	var elbs *elb.DescribeLoadBalancersOutput
	var instances *ec2.DescribeInstancesOutput
	var networks *ec2.DescribeSubnetsOutput

	wg.Add(1)
	go func() {
		var err error
		instances, err = getInstances(cfg, instanceCount)
		if err != nil {
			fmt.Println(err)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		var err error
		secGroups, err = getSecGroups(cfg)
		if err != nil {
			fmt.Println(err)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		var err error
		elbs, err = getElbs(cfg)
		if err != nil {
			fmt.Println(err)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		var err error
		networks, err = getSubnets(cfg)
		if err != nil {
			fmt.Println(err)
		}
		wg.Done()
	}()

	wg.Wait()

	var err error

	target := os.Getenv("HOME") + "/.awsmap"
	err = os.Mkdir(target, 0755)
	if err != nil && os.IsNotExist(err) {
		log.Println("mkdir: " + err.Error())
		os.Exit(1)
	}

	fmode := os.O_TRUNC | os.O_CREATE | os.O_WRONLY

	if secGroups != nil {
		// TODO: (NF 2015-05-29) prefix with account or something
		f, err := os.OpenFile(target+"/sg.json", fmode, 0644)
		defer f.Close()
		if err != nil {
			log.Println("sg write: " + err.Error())
			return
		}

		err = writeJson(secGroups, f)
		if err != nil {
			fmt.Println(err)
			// TODO: (NF 2015-05-29) unlink the file and shit.
		}
	}

	if elbs != nil {
		f, err := os.OpenFile(target+"/elbs.json", fmode, 0644)
		defer f.Close()
		if err != nil {
			log.Println("elb write: " + err.Error())
			return
		}

		err = writeJson(elbs, f)
		if err != nil {
			fmt.Println(err)
		}
	}

	if instances != nil {
		f, err := os.OpenFile(target+"/instances.json", fmode, 0644)
		defer f.Close()
		if err != nil {
			log.Println("instance write: " + err.Error())
			return
		}

		err = writeJson(instances, f)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	if networks != nil {
		err = writeJson(networks, os.Stdout)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func main() {
	var region string
	var instanceCount int64
	var isOnline bool

	flag.BoolVar(&isOnline, "online", false, "Retrieve latest data.")
	flag.Int64Var(&instanceCount, "instances", 100, "Number of running instances.")
	flag.StringVar(&region, "region", "eu-west-1", "AWS region to map.")
	flag.Parse()

	if isOnline {
		online(region, instanceCount)
	}
}
