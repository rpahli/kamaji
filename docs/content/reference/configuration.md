## Configuration

Currently **Kamaji** supports (in this order):

* CLI flags
* Environment variables
* Configuration files

By default **Kamaji** search for the configuration file (`kamaji.yaml`) and uses parameters found inside of it. In case some environment variable are passed, this will override configuration file parameters. In the end, if also a CLI flag is passed, this will override both env vars and config file as well.

This is easily explained in this way:

`cli-flags` > `env-vars` > `config-files`

Available flags are the following:

```
--config-file string                 Configuration file alternative. (default "kamaji.yaml")
--datastore string                   The default DataStore that should be used by Kamaji to setup the required storage (default "etcd")
--health-probe-bind-address string   The address the probe endpoint binds to. (default ":8081")
--kine-image string                  Container image along with tag to use for the Kine sidecar container (used only if etcd-storage-type is set to one of kine strategies) (default "rancher/kine:v0.9.2-amd64")
--kubeconfig string                  Paths to a kubeconfig. Only required if out-of-cluster.
--leader-elect                       Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
--metrics-bind-address string        The address the metric endpoint binds to. (default ":8080")
--tmp-directory string               Directory which will be used to work with temporary files. (default "/tmp/kamaji")
--zap-devel                          Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
--zap-encoder encoder                Zap log encoding (one of 'json' or 'console')
--zap-log-level level                Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
--zap-stacktrace-level level         Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
--zap-time-encoding time-encoding    Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.
```

Available environment variables are:

| Environment variable                 | Description                                                                                                            |
|--------------------------------------|------------------------------------------------------------------------------------------------------------------------|
| `KAMAJI_DATASTORE`                   | Name of the DataStore resource with driver definition and settings. (default "etcd")                                   |
| `KAMAJI_METRICS_BIND_ADDRESS`        | The address the metric endpoint binds to. (default ":8080")                                                            |
| `KAMAJI_HEALTH_PROBE_BIND_ADDRESS`   | The address the probe endpoint binds to. (default ":8081")                                                             |
| `KAMAJI_LEADER_ELECTION`             | Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.  |
| `KAMAJI_TMP_DIRECTORY`               | Directory which will be used to work with temporary files. (default "/tmp/kamaji")                                     |