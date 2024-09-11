source "amazon-ebs" "generic" {
  ami_name      = "generic-{{timestamp}}"
  instance_type = var.instance_type 
  region        = var.region
  source_ami    = var.ami_id 
  ssh_username  = "ec2-user"
  communicator  = "ssh"

  launch_block_device_mappings {
    device_name           = "/dev/xvda"
    volume_size           = 50
    volume_type           = "gp3"
    delete_on_termination = true
  }


}

build {
  name    = "generic"
  sources = ["source.amazon-ebs.generic"]
  
  provisioner "shell" {
    inline = [
      "sudo dnf -y install crypto-policies crypto-policies-scripts",
      "sudo fips-mode-setup --enable",
      "sudo reboot"
    ]
  }

  provisioner "shell" {
    inline = [
      "sudo fips-mode-setup --check"
    ]
  }
}
