# EC2 SSH config generator

Generate a ssh config file for your aws ec2 instances.

The binary will generate a file ``ec2_cfg.config`` in defined ssh path.

### How to run it
Example:

``
go build -o {outputfile} .
AWS_ACCESS_KEY=xxx AWS_SECRET_KEY=xxx AWS_REGION=eu-central-1 SSH_CONFIG_PATH=~/.ssh/ec2_cfg.config SSH_KEY_PATH=~/.ssh/id_ed25519 \
./{outputfile}
``
The output will be the file in ``~/.ssh/ec2-cfg.config`` with a content:
```
Host instance-527f9a0
	Hostname 3.01.111.222
	IdentityFile /Users/user/.ssh/id_ed25519
	User ec2-user
```

### Required environment variables

- AWS_ACCESS_KEY
- AWS_SECRET_KEY
- AWS_REGION
- SSH_CONFIG_PATH
- SSH_KEY_PATH ( path to ssh key pair )


### How to use

After generation a file use command:

```
// put a correct path to generated config file
ssh -F ~/.ssh/ec2_cfg.config instance-527f9a0
```

As alternative, just put the content of the file to main ``.ssh/config`` file.
