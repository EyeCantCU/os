# vms-test
This repository houses a test harness for Chainguard vms.


## AWS
Example of using aws.


 * Find an image id

      ```
      $ aws ec2 describe-images --region=us-west-2 --executable-users=self > out.json
      $ jq -r '.Images[] | select(.Name? | match("chainguard-base")) | "\(.ImageId) \(.Name)"' > out
      $ sort -k2 out | tail -n 3
      ami-07dc0f4cce9d9a30f chainguard-base-x86_64-20250402-1742-prod-pprbi7od3pt3q
      ami-0ca028505645841eb chainguard-base-x86_64-20250402-2354-prod-pprbi7od3pt3q
      ami-0ac7483330414d688 chainguard-base-x86_64-20250403-1617-prod-pprbi7od3pt3q
      ```

 * Setup necessary resources (one time setup)

      ```
      $ ./out/ec2  setup --region=us-west-2 --tag=smoser-test
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
     $ ./out/ec2 launch --region=us-west-2 --tag=smoser-dev1 --public-key=./id_ed25519.pub \
         --subnet-id=subnet-0f9ec4068399fa4bf \
         --vpc-id=vpc-07fa6bb7b6a4d1ec2 \
         --security-group-id=sg-09e97361cc8c787db \
         --instance-type=t3.medium \
         --ami-id=ami-0ac7483330414d688 
     2025/04/03 15:27:29 Launched instance ID: i-05ce463388c21c51a
     ```

 * ssh 

     ```
     $ ./out/ec2 ssh --region=us-west-2 --tag=smoser-dev1
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
    $ ./out/ec2 terminate --region=us-west-2 --tag=smoser-dev1
    2025/04/03 15:30:51 Requested termination for instances: [i-05ce463388c21c51a]
    2025/04/03 15:30:51 Deleted key pair: key-smoser-dev1
    ```



