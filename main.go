package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_config "github.com/aws/aws-sdk-go-v2/config"
	ec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"golang.org/x/sync/errgroup"
)

// Defined required env variables
var (
	ENV_AWS_ACCESS_KEY  = "AWS_ACCESS_KEY"
	ENV_AWS_SECRET_KEY  = "AWS_SECRET_KEY"
	ENV_SSH_CONFIG_PATH = "SSH_CONFIG_PATH"
	ENV_SSH_KEY_PATH    = "SSH_KEY_PATH"
)

func loadEnvConfig() (key, secret, sshPath, sshKeyPath string) {
	key = os.Getenv(ENV_AWS_ACCESS_KEY)
	secret = os.Getenv(ENV_AWS_SECRET_KEY)
	sshPath = os.Getenv(ENV_SSH_CONFIG_PATH)
	sshKeyPath = os.Getenv(ENV_SSH_KEY_PATH)
	if key == "" || secret == "" || sshPath == "" {
		log.Fatal(fmt.Sprintf("check passing the required env variables"))
	}
	return
}

//go:embed help.txt
var fileHelpContent string

func loadFlags() {
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

func describeInstance(ctx context.Context, ec2Client *ec2.Client, instance types.Instance, in chan<- ec2InstanceShort) {
	res, err := ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{*instance.ImageId},
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(res.Images) == 0 {
		log.Fatal("no images assigned to instance")
	}

	image := res.Images[0]
	_, _, _, keyPair := loadEnvConfig()
	in <- ec2InstanceShort{
		Key:         aws.ToString(instance.KeyName),
		User:        getEC2user(aws.ToString(image.Description)),
		IP:          aws.ToString(instance.PublicIpAddress),
		KeyPairPath: keyPair,
	}
}

func describeInstances(ctx context.Context, cfg aws.Config) (<-chan ec2InstanceShort, error) {
	ec2InstancesCh := make(chan ec2InstanceShort)

	client := ec2.NewFromConfig(cfg)

	output, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, err
	}

	for _, r := range output.Reservations {
		for _, instance := range r.Instances {
			go describeInstance(ctx, client, instance, ec2InstancesCh)
		}
	}

	return ec2InstancesCh, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errGroup, ctx := errgroup.WithContext(ctx)
	loadFlags()

	cfg, err := aws_config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create the instances channel
	instancesInfoCh, err := describeInstances(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Run the writing to a resulting file.
	errGroup.Go(func() error {
		_, _, ssh_path, _ := loadEnvConfig()
		return writeToFile(ctx, ssh_path, instancesInfoCh)
	})

	if err := errGroup.Wait(); err != nil {
		log.Fatal(err)
	}

	successMessage()
}

func successMessage() {
	_, _, ssh_path, _ := loadEnvConfig()
	fmt.Printf(`
All done.
Check the created file ec2_aws.config in %s directory.
\n`, ssh_path)
}

type userOs struct {
	u  string
	os []string
}

var (
	amazon   = userOs{"ec2-user", []string{"amazon", "amzn"}}
	ubuntu   = userOs{"ubuntu", []string{"ubuntu"}}
	debian   = userOs{"admin", []string{"debian"}}
	centos   = userOs{"centos", []string{"centos"}}
	defaultU = userOs{"ec2-user", []string{}}
)

func getEC2user(imageDescription string) string {
	for _, uos := range []userOs{
		amazon,
		ubuntu,
		debian,
		centos,
	} {
		if containsAny(imageDescription, uos.os...) {
			return uos.u
		}
	}
	return defaultU.u
}

func containsAny(s string, substr ...string) bool {
	for _, p := range substr {
		if strings.Contains(strings.ToLower(s), strings.ToLower(p)) {
			return true
		}
	}
	return false
}
