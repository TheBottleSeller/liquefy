package executor

import (
	"errors"
	"os/exec"
	"strings"

	log "github.com/Sirupsen/logrus"
)

const UVM_DEVICE = "/dev/nvidia-uvm"
const CTL_DEVICE = "$/dev/nvidia-ctl"

const CUDA_VERSION_LABEL = "lq.nvidia.cuda.version"

const NV_BINS_VOLUME = "/usr/local/bin"

var NV_BINS = []string{"nvidia-cuda-mps-control", "nvidia-cuda-mps-server", "nvidia-debugdump", "nvidia-persistenced", "nvidia-smi"}

const NV_LIBS_VOLUME = "/usr/local/nvidia"

var NV_LIBS_CUDA = []string{"cuda", "nvcuvid", "nvidia-compiler", "nvidia-encode", "nvidia-ml"}

//__nvsmi_query()
//{
//local section="$1"
//local gpu_id="$2" # optional
//local cmd="nvidia-smi -q"
//
//if [ $# -eq 2 ]; then
//cmd="$cmd -i $gpu_id"
//fi
//echo $( $cmd | grep "$section" | awk '{print $4}' )
//}

//__library_paths()
//{
//local lib="$1"
//echo $( ldconfig -p | grep "lib${lib}.so" | awk '{print $4}' )
//}

//__library_arch()
//{
//local lib="$1"
//
//echo $( file -L $lib | awk '{print $3}' | cut -d- -f1 )
//}

//__filter_duplicate_paths()
//{
//local paths="$1"
//
//local sums="$( md5sum $paths | sed 's/[^/]*$/ &/' )"
//local uniq="$( echo "$sums" | uniq -u -f2 | awk '{print $2$3}')"
//local dupl="$( echo "$sums" | uniq --all-repeated=separate -f2 \
//| uniq -w 32 | awk 'NF {print $2$3}')"
//echo $uniq $dupl
//}
//

//check_prerequisites(){
//	local cmds="nvidia-smi nvidia-modprobe"
//  for cmd in $cmds; do
//		command -v $cmd >/dev/null && continue
//		__log ERROR "Command not found: $cmd"
//		exit 1
//	done
//}

func check_prerequisites() error {
	cmds := []string{"nvidia-smi", "nvidia-modprobe"}
	for _, cmd := range cmds {
		if out, err := exec.Command(cmd).Output(); string(out) == "" || err != nil {
			return errors.New("Nvidia Prereqs Failed")
		}
	}
	return nil
}

//__nvsmi_query(){
//	local section="$1"
//	local gpu_id="$2" # optional
//	local cmd="nvidia-smi -q"
//
//	if [ $# -eq 2 ]; then
//		cmd="$cmd -i $gpu_id"
//	fi
//	echo $( $cmd | grep "$section" | awk '{print $4}' )
//}
func nvsmi_query(section string, gpuid int) (string, error) {
	out, err := exec.Command("nvidia-smi -q -i " + string(gpuid) + "| grep " + section + " | awk '{print $4}' ").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

//load_uvm()
//{
//if [ ! -e $UVM_DEVICE ]; then
//nvidia-modprobe -u -c=0
//fi
//}

func load_uvm() {

}

func constructdevices() {
	//Constrct Devices
	//local args="--device=$CTL_DEVICE --device=$UVM_DEVICE"

	//Add Device for GPU ( Get
	//local minor="$( __nvsmi_query "Minor Number" $gpu )"
	//args="$args --device=${NV_DEVICE}$minor"

}

func libraryMounts() {

	//for lib in $NV_LIBS_CUDA; do
	//local paths="$( __library_paths $lib )"
	//if [ -z "$paths" ]; then
	//__log WARN "Could not find library: $lib"
	//continue
	//fi

	//	for path in $( __filter_duplicate_paths "$paths" ); do
	//args="$args -v $path:$path"
	//case $( __library_arch "$path" ) in
	//32) args="$args -v $path:$NV_LIBS_VOLUME/lib/$(basename $path)" ;;
	//64) args="$args -v $path:$NV_LIBS_VOLUME/lib64/$(basename $path)" ;;
	//esac
	//done
	//done

	for _, e := range NV_LIBS_CUDA {
		out, err := exec.Command("ldconfig -p | grep \"lib " + e + ".so\" | awk '{print $4}' ").Output()
		if err != nil {
			log.Warn(err)
		} else {
			found := make(map[string]bool)
			paths := strings.Split(string(out), "\n")
			for _, path := range paths {
				if !found[path] {
					//Mount Path
					//Mount LIBS VOLUME
					found[path] = true
				}
			}
		}
	}

}

func binMounts() error {

	//	for bin in $NV_BINS; do
	//	local path="$( which $bin )"
	//	if [ -z $path ]; then
	//__log WARN "Could not find binary: $bin"
	//continue
	//fi
	//args="$args -v $path:$NV_BINS_VOLUME/$bin"
	//done

	for bin := range NV_BINS {
		_, err := exec.Command("whcih " + string(bin)).Output()
		if err != nil {
			return err
		}
		//Mount -v thing herer
	}
	return nil
}
