import pyinotify
import subprocess
import sys

class ProcessTransientFile(pyinotify.ProcessEvent):

    def process_IN_MODIFY(self, event):
        sys.stdout.write(f'\t {event.maskname} -> written')

    def process_IN_DELETE(self, event):
        sys.stdout.write(f'\t {event.maskname} -> delete')
        exit(1)

    def process_IN_CREATE(self, event):
        sys.stdout.write(f'\t {event.maskname} -> create')

    def process_IN_CLOSE_WRITE(self, event):
        sys.stdout.write(f'\t {event.maskname} -> in_close_write')

    def process_IN_MOVED_TO(self, event):
        sys.stdout.write(f'\t {event.maskname} -> in_moved_to')

    def process_default(self, event):
        sys.stdout.write(f'default: {event.maskname}')

def get_ofport(input):
    cmd = "ovs-vsctl", "--bare", "--columns=ofport", "list", "int", input 
    result = subprocess.run(cmd, timeout=15, capture_output=True, text=True)
    if result.returncode:
        exit(1)
    return result.stdout.splitlines()[0]

def add_flows(input, output):
    cmd = "ovs-ofctl", "add-flow", "br-hbn", "in_port=%s,actions=%s" % (input, output)
    print(cmd)
    result = subprocess.run(cmd, timeout=15, capture_output=True, text=True)
    if result.returncode:
        exit(1)
    cmd = "ovs-ofctl", "add-flow", "br-hbn", "in_port=%s,actions=%s" % (output, input)
    print(cmd)
    result = subprocess.run(cmd, timeout=15, capture_output=True, text=True)
    if result.returncode:
        exit(1)
    return

print("Starting")
# Get a list of all dpdk ports in the bridge br-hbn
result = subprocess.run(["ovs-vsctl", "list-ports", "br-hbn"], timeout=15, capture_output=True, text=True)
if result.returncode:
    exit(1)
items = result.stdout.splitlines()
print(items)
for port in items:
    if "brhbn" not in port:
        input = get_ofport(port)
        output = get_ofport("p%sbrhbn" % port)
        add_flows(input, output)
wm = pyinotify.WatchManager()
notifier = pyinotify.Notifier(wm)
# Monitor the file /var/run/openvswitch/ovs-vswitchd.pid.
# We assume that the file exists (ovs-vswitchd is running)
# and we exit with an return code of 1 in the case
# IN_CLOSE_WRITE (ovs-vswitchd restarted) was signaled
wm.watch_transient_file('/var/run/openvswitch/ovs-vswitchd.pid', pyinotify.ALL_EVENTS, ProcessTransientFile)
notifier.loop()

