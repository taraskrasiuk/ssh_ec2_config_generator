package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	aws_config "github.com/aws/aws-sdk-go-v2/config"
	ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"

	"golang.org/x/sync/errgroup"
)

//go:embed help.txt
var fileHelpContent string

var (
	key        = os.Getenv("AWS_ACCESS_KEY")
	secret     = os.Getenv("AWS_SECRET_KEY")
	region     = os.Getenv("AWS_REGION")
	sshPath    = os.Getenv("SSH_CONFIG_PATH")
	sshKeyPath = os.Getenv("SSH_KEY_PATH")
)

func validateEnv() error {
	if key == "" || secret == "" || sshPath == "" || region == "" {
		return errors.New("check passing the required env variables")
	}
	return nil
}

func flagUsage() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, fileHelpContent)
	}
	flag.Parse()
}

type ec2InstanceShort struct {
	Key         string
	User        string
	IP          string
	KeyPairPath string
}

// Return a string in format:
// Host ec2Instance
//
//	Hostname 32.0.0.1
//	IdentityFile /usr/User/.ssh/rsa1
//	User ec2-user
func (e ec2InstanceShort) ToString() string {
	return fmt.Sprintf("Host %s\n\tHostname %s\n\tIdentityFile %s\n\tUser %s\n", e.Key, e.IP, e.KeyPairPath, e.User)
}

func writeToFile(ctx context.Context, path string, out <-chan ec2InstanceShort) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case info := <-out:
		{
			_, err := f.Write([]byte(info.ToString()))
			return err
		}
	}
}

func describeInstance(ctx context.Context, ec2Client *ec2.Client, in chan<- ec2InstanceShort, imageId, keyName, publicIpAddr string) error {
	res, err := ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{imageId},
	})
	if err != nil {
		return err
	}
	if len(res.Images) == 0 {
		return errors.New("no images assigned to instance")
	}

	image := res.Images[0]
	in <- ec2InstanceShort{
		Key:         keyName,
		User:        getEC2user(*image.Description),
		IP:          publicIpAddr,
		KeyPairPath: sshKeyPath,
	}
	return nil
}

// Generator function, which retrieve the information about all ec2 instances.
// Returning the channel, which going to be filled with instance short information.
func instanceInfoGenerator(ctx context.Context) (<-chan ec2InstanceShort, error) {
	ec2InstancesCh := make(chan ec2InstanceShort)
	// create aws config
	cfg, err := aws_config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	// create a ec2 client
	client := ec2.NewFromConfig(cfg)

	// retrieve the information about ec2 instances.
	// TODO: add pagination
	output, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, err
	}
	// go through the output and run each instance in go routing, for gathering required information
	for _, r := range output.Reservations {
		for _, instance := range r.Instances {
			go describeInstance(ctx, client, ec2InstancesCh, *instance.ImageId, *instance.KeyName, *instance.PublicIpAddress)
		}
	}

	return ec2InstancesCh, nil
}

func main() {
	// load helper flag
	flagUsage()

	// validate provided env variables
	if err := validateEnv(); err != nil {
		log.Fatal(err)
	}

	// create a context with timeout for 5 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errGroup, ctx := errgroup.WithContext(ctx)

	errGroup.Go(func() error {
		// Run the instance function generator and retrive a channel
		instancesInfoShortCh, err := instanceInfoGenerator(ctx)
		if err != nil {
			return err
		}
		// Use an instancesInfo channel and write to final file
		return writeToFile(ctx, sshPath, instancesInfoShortCh)
	})

	// check for errors
	if err := errGroup.Wait(); err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(os.Stdout, `
All done.
Check the created file ec2_aws.config in %s directory.
\n`, sshPath)
}
