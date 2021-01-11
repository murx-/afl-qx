# afl Queue eXplorer - afl-qx

afl-qx is a queue/output folder explorer for afl(++) instances.

## Usage

```
$ go run afl-qx.go  -in <afl_out_dir> -listen <address>:<port>
```

Keep in mind that AFL++3.00 changed the default behavior an every fuzzer is now a named instance. E.g.

```
$ go run afl-qx.go  -in out/default -listen localhost:8080
```

Once running you can explorer edges to show a diff of the inputs showing the mutation that was perfomred. You can also explorer the nodes which will give you a hexdump of the testcase. 

Legend:

- Green nodes and edges mean new coverage was reached
- Blue nodes and edges mean that the hit count was updated
- Orange nodes and edges mean that a hang was identified
- Red nodes and endes mean that a crash was identified

## Examples

![Diff View](https://github.com/murx-/afl-qx/blob/main/images/diff.png)

![Hexdump View](https://github.com/murx-/afl-qx/blob/main/images/hexdump.png)
