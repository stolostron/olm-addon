# OLM everywhere!

This directory contains a demo of the use of the OLM-addon.

## Pre-requisites

- [kind](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [yq](https://mikefarah.gitbook.io/yq/)
- enough OS file watches, e.g.:
~~~
$ sudo sysctl -w fs.inotify.max_user_watches=2097152
$ sudo sysctl -w fs.inotify.max_user_instances=256
~~~
- an IP address (preferably private) the node ports can be bound to. 127.0.0.1 is not suitable for cross cluster communication.
- the terminal tool [Pipe Viewer (pv)](http://www.ivarch.com/programs/pv.shtml")

## Setup

The environment can get prepared by using [the setup script](./setup.sh).
It accepts the following parameters

- "-a < IP >": the IP node ports are bound to, default is 192.168.130.1
- "-p < HTTP_PORT >": the starting port for http. Subsequent clusters use the starting port incremented by 100, default is 8080
- "-t < TLS_PORT >": the starting port for https. Subsequent clusters use the starting port incremented by 100, default is 8443

~~~
./demo/setup.sh
~~~

## Run

Once the setup has been completed the demo can simply be run with:

~~~
./demo/run.sh
~~~

Use "Enter" to get the next comment or command. This is live! You can access the clusters at any time from a different terminal using the kubeconfig available in the `./demo/.demo` directory. You can then look at the resources in the clusters in more details, if you wish, and resume the demo in the first terminal when you are ready.

## Cleanup

To clean up the demo simply remove the kind clusters:

~~~
kind delete clusters hub spoke1 spoke2
~~~