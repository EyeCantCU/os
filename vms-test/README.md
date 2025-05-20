# vms-test
This repository houses a test harness for Chainguard vms.

# Build test runners

```
make runner/aws
```

## AWS
Login / setup aws client:

 * download and install [awscli](https://aws.amazon.com/cli/) aws client
 * login with client
     ```
     $ aws configure sso
     SSO session name (Recommended): my-dev
     There are 13 AWS accounts available to you
     > select the 'Dev' account
     > CLI default client Region [None]: us-west-2
     > CLI default output format [None]:
     > CLI profile name [Engineering-xxxxx-xxxxxxxxxxxx]: my-profile
     ```
 * set `AWS_DEFAULT_PROFILE` in environment
     ```
     $ export AWS_DEFAULT_PROFILE=my-profile
     ```

Example of using aws.


 * Find an image id

      ```
      $ aws ec2 describe-images --region=us-west-2 --executable-users=self > out.json
      $ jq -r '.Images[] | select(.Name? | match("chainguard-base")) | "\(.ImageId) \(.Name)"' <out.json > out
      $ sort -k2 out | tail -n 3
      ami-07dc0f4cce9d9a30f chainguard-base-x86_64-20250402-1742-prod-pprbi7od3pt3q
      ami-0ca028505645841eb chainguard-base-x86_64-20250402-2354-prod-pprbi7od3pt3q
      ami-0ac7483330414d688 chainguard-base-x86_64-20250403-1617-prod-pprbi7od3pt3q
      ```

 * Setup necessary resources (one time setup)

      ```
      $ ./runner/aws  setup --region=us-west-2 --tag=smoser-test
      2025/04/03 15:24:49 Created VPC: vpc-07fa6bb7b6a4d1ec2
      2025/04/03 15:24:49 Created Subnet: subnet-0f9ec4068399fa4bf
      2025/04/03 15:24:49 Created Security Group: sg-09e97361cc8c787db
      2025/04/03 15:24:50 Created Internet Gateway: igw-09abc20bba35ac3ea
      2025/04/03 15:24:50 Attached IGW to VPC
      2025/04/03 15:24:50 Created Route Table: rtb-065b05fc52c01d187
      2025/04/03 15:24:50 Added default route through IGW
      2025/04/03 15:24:51 Associated route table with subnet
      2025/04/03 15:24:51 Setup complete.
      ```

 * Launch an instance

     ```
     $ ./runner/aws launch --region=us-west-2 --tag=smoser-dev1 --public-key=./id_ed25519.pub \
         --subnet-id=subnet-0f9ec4068399fa4bf \
         --vpc-id=vpc-07fa6bb7b6a4d1ec2 \
         --security-group-id=sg-09e97361cc8c787db \
         --instance-type=t3.medium \
         --ami-id=ami-0ac7483330414d688 
     2025/04/03 15:27:29 Launched instance ID: i-05ce463388c21c51a
     ```

 * ssh 

     ```
     $ ./runner/aws ssh --region=us-west-2 --tag=smoser-dev1
     ip-10-0-0-50:~$ ls         
     ls
     ip-10-0-0-50:~$ ls /     
     bin    etc    opt    sbin   var
     boot   home   proc   sys
     dev    lib    root   tmp
     efi    lib64  run    usr
     ip-10-0-0-50:~$ exit       

 * terminate

    ```
    $ ./runner/aws terminate --region=us-west-2 --tag=smoser-dev1
    2025/04/03 15:30:51 Requested termination for instances: [i-05ce463388c21c51a]
    2025/04/03 15:30:51 Deleted key pair: key-smoser-dev1
    ```

## Azure

Example of using azure.


 * Find an image id.

      You will need to provide the gallery name and image name at a minimum. If
      the resource group of the gallery is different from the one you will run the
      tests in, you will need to provide that as well. If the subscription ID is
      different from `az account show | jq -r .id` you will need to provide that as well.

      ```
      $ az sig list | jq '[.[] | {name,resourceGroup}]' # Find a gallery
      [
        {
          "name": "vmtesting",
          "resourceGroup": "CHAINGUARD-VMS",
          "id": "/subscriptions/ad60e736-b0ce-432e-b77c-4b8218f452ae/resourceGroups/CHAINGUARD-VMS/providers/Microsoft.Compute/galleries/vmtesting"
        }
      ]
      $ az sig image-definition list -g chainguard-vms -r vmtesting | jq '[.[] | {name} ] | unique'
      [
        {
          "name": "chainguard-agents-arm64"
        },
        {
          "name": "chainguard-agents-x64"
        }
      ]
      ```

 * Setup necessary resources (one time setup)

      ```
      $ AZ_ACCOUNT_ID=$(az account show | jq -r .id)
      $ echo $AZ_ACCOUNT_ID
      ad60e736-b0ce-432e-b77c-4b8218f452ae

      $ ./runner/azure setup --nsg-name acrate-testrunner-nsg --region eastus \
          --resource-group chainguard-vms --subscription-id=${AZ_ACCOUNT_ID} \
          --tag acrate-testrunner --vnet-name acrate-testrunner-vnet --route-table-name acrate-testrunner-routetable \
          --subnet-name acrate-testrunner-subnet
      2025/04/07 15:38:52 Using existing resource group: chainguard-vms
      2025/04/07 15:38:53 Created NSG: /subscriptions/ad60e736-b0ce-432e-b77c-4b8218f452ae/resourceGroups/chainguard-vms/providers/Microsoft.Network/networkSecurityGroups/acrate-testrunner-nsg
      2025/04/07 15:38:54 Created Route Table: /subscriptions/ad60e736-b0ce-432e-b77c-4b8218f452ae/resourceGroups/chainguard-vms/providers/Microsoft.Network/routeTables/acrate-testrunner-routetable
      2025/04/07 15:39:00 Created VNet: /subscriptions/ad60e736-b0ce-432e-b77c-4b8218f452ae/resourceGroups/chainguard-vms/providers/Microsoft.Network/virtualNetworks/acrate-testrunner-vnet
      2025/04/07 15:39:05 Created Subnet with NSG and Route Table association: /subscriptions/ad60e736-b0ce-432e-b77c-4b8218f452ae/resourceGroups/chainguard-vms/providers/Microsoft.Network/virtualNetworks/acrate-testrunner-vnet/subnets/acrate-testrunner-subnet
      2025/04/07 15:39:05 Azure network setup complete.
      ```

 * Launch an instance

     ```
     $ ./runner/azure launch --subscription-id=${AZ_ACCOUNT_ID} --tag acrate-testrunner \
         --resource-group chainguard-vms --name acrate-testrunner-vm-3 --location eastus --public-key ~/.ssh/azure.pub \
         --subnet acrate-testrunner-subnet --vnet acrate-testrunner-vnet --image-gallery vmtesting --image-name chainguard-agents-x64 \
         --image-version latest
     2025/04/08 12:07:17 Creating public IP: ip-acrate-testrunner-vm-3
     2025/04/08 12:07:31 Launched Azure VM: acrate-testrunner-vm-3
     ```

 * ssh 

     ```
     $ ./runner/azure ssh --subscription-id=${AZ_ACCOUNT_ID} --tag acrate-testrunner --resource-group chainguard-vms \
         --private-key ~/.ssh/azure
     2025/04/08 12:10:39 Connecting to VM at 172.190.50.229
     acrate-testrunner-vm-3:~$ uname -a
     uname -a
     Linux acrate-testrunner-vm-3 6.14.0-r4-azure-generic #Chainguard SMP Thu Mar 27 17:26:34 UTC 2025 x86_64 GNU/Linux
     acrate-testrunner-vm-3:~$ exit
     ```

 * terminate

    ```
    $ ./runner/azure terminate --subscription-id=${AZ_ACCOUNT_ID} --tag acrate-testrunner --resource-group chainguard-vms
    2025/04/08 12:12:28 Deleting VM: acrate-testrunner-vm-3
    2025/04/08 12:13:13 Deleted VM: acrate-testrunner-vm-3
    2025/04/08 12:13:14 Deleting disk: acrate-testrunner-vm-3_OsDisk_1_d23070c08c1a481e8b039ef913b15d4f
    2025/04/08 12:13:16 Deleted disk: acrate-testrunner-vm-3_OsDisk_1_d23070c08c1a481e8b039ef913b15d4f
    2025/04/08 12:13:19 Deleting NIC: nic-acrate-testrunner-vm-3
    2025/04/08 12:13:25 Deleted NIC: nic-acrate-testrunner-vm-3
    2025/04/08 12:13:27 Deleting public IP: ip-acrate-testrunner-vm-3
    2025/04/08 12:14:00 Deleted public IP: ip-acrate-testrunner-vm-3
    ```

 * teardown

    ```
    $ ./runner/azure teardown --subscription-id=${AZ_ACCOUNT_ID} --tag acrate-testrunner --resource-group chainguard-vms
    2025/04/08 12:15:47 Deleting vNet: acrate-testrunner-vnet
    2025/04/08 12:16:02 Deleted vNet: acrate-testrunner-vnet
    2025/04/08 12:16:02 Deleting NSG: acrate-testrunner-nsg
    2025/04/08 12:16:06 Deleted NSG: acrate-testrunner-nsg
    2025/04/08 12:16:06 Deleting route table: acrate-testrunner-routetable
    2025/04/08 12:16:12 Deleted route table: acrate-testrunner-routetable
    ```

## GCE

Example of using GCE

  * Authenticate with application default credentials

    ```
    $ gcloud auth login --update-adc
    ```

  * Launch an instance

    ```
    $ ./runner/gce launch --label test --name test-vm --project zmarano-chainguard --public-key ~/.ssh/id_ed25519.pub --source-image-uri projects/wolfi-vm/global/images/family/chainguard-docker-dev-amd64 
    2025/04/11 09:12:27 Launched GCE VM: test-vm
    ```

  * SSH - TODO the terminal isn't quite right.

    ```
    $ ./runner/gce ssh --name test-vm --project zmarano-chainguard --private-key ~/.ssh/id_ed25519
    2025/04/11 09:19:25 Connecting to VM at 34.68.213.163
    test-vm:~# ^[[41;12R
    ```

  * Execute a file

    ```
    $ ./runner/gce run-remote --name test-vm --project zmarano-chainguard --private-key ~/.ssh/id_ed25519 --file /tmp/hello.sh
    2025/04/11 09:18:41 Connecting to VM at 34.68.213.163
    Hello World!
    ```

  * Terminate

    Note: In GCE, using deafult instance create and delete behavior, the boot disk is also deleted with the request. We aren't yet creating networks per test and thus the default VPC in the project is being used. There is therefore no additional cleanup in this basic flow.

    ```
    $ ./runner/gce terminate --name test-vm --project zmarano-chainguard
    # This will currently fail because wolfi isn't responding to ACPI events. Fix in progress.
    2025/04/11 09:22:47 instance deletion operation failed: context deadline exceeded
    ```
