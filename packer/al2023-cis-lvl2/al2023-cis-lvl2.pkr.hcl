packer {
  required_plugins {
    amazon = {
      version = ">= 1.2.1"
      source  = "github.com/hashicorp/amazon"
    }
    ansible = {
      version = ">= 1.1.0"
      source  = "github.com/hashicorp/ansible"
    }
  }
}
variable "ami_id" {
  type    = string
  default = "ami-0c2b8ca1dad447f8a"
}

variable "instance_type" {
  type    = string
  default = "c6i.large"
}

variable "region" {
  type    = string
  default = "us-west-2"
}

variable "ami_name" {
  type    = string
  default = "hardened-al2023-cis-lvl2-eks-{{timestamp}}"
}

source "amazon-ebs" "al2023" {
  region        = var.region
  instance_type = var.instance_type
  ssh_username  = "ec2-user"
  ami_name      = var.ami_name
  source_ami    = var.ami_id

  tags = {
    Name        = "Hardened-AL2023-CIS"
    Environment = "Production"
    Hardened    = "Yes"
    CISVersion  = "1.0"
  }

  force_delete_snapshot = true
}

build {
  sources = ["source.amazon-ebs.al2023"]

  provisioner "shell" {
    inline = [
      "sudo dnf update -y",
      "sudo dnf install -y ansible git openssl aide firewalld rsyslog acl",
      "ansible-galaxy install git+https://github.com/ansible-lockdown/AMAZON2023-CIS.git",
    ]
    execute_command = "{{.Vars}} bash '{{.Path}}'"
  }

 provisioner "shell" {
    inline = [
      "sudo dnf -y install crypto-policies crypto-policies-scripts",
      "sudo fips-mode-setup --enable",
      
    ]
    expect_disconnect = true
    execute_command   = "{{.Vars}} bash '{{.Path}}'"
  }

  provisioner "ansible-local" {
    playbook_file   = "packer/al2023-cis-lvl2/playbook.yml"
    extra_arguments = ["-v", "--extra-vars", "ansible_python_interpreter=/usr/bin/python3"]
  }

  provisioner "shell" {
    inline = [
      "sudo reboot",
      "sudo fips-mode-setup --check"
    ]
    execute_command   = "{{.Vars}} bash '{{.Path}}'"
    expect_disconnect = true
  }
 
}
