# qemu test

## Create a new run directory

    $ ./runner/qemu create --firmware=./builder/ovmf-x86_64.fd my-vmdir/ disk.img
    $ ls my-vmdir
    disk.img
    fw-code.fd

Creating dir, I have also used the committed script like this:


    $ for a in x86_64 aarch64; do
       ./pkg/runner/qemu/scripts/vmd --backdoor ~/src/wolfi-vm my-$a generic $a ; done


## Start

    $ ./runner/qemu start --public-key=$HOME/.ssh/id_ed25519.pub my-x86_64/ &
    $ ls my-vmdir
    console.log
    console.sock
    disk.img
    fw-code.fd
    hmp.sock
    qmp.sock
    ttyS1.sock

## ssh

    $ ./runner/qemu ssh -i ~/.ssh/id_ed25519 ./my-x86_64/ cat /proc/uptime
    2025/04/17 14:25:23 connecting to 127.0.0.1:43515
    4184.15 4177.70

## terminate

this is TBD.  I just ctrl-c the Start provcess
