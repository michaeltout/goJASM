package main

import (
	"io"
	"os"

	"git.practool.xyz/nova/goJASM/ijvmasm"
	"git.practool.xyz/nova/goJASM/opconf"
	"github.com/op/go-logging"
	flag "github.com/spf13/pflag"
	"fmt"
)

var log *logging.Logger
var flagInfo bool
var flagDebug bool
var flagConfig string
var flagOutput string
var flagForce bool
var flagAutoWide bool

func init() {
	flag.BoolVarP(&flagInfo, "info", "i", false, "enable info message logging")
	flag.BoolVarP(&flagDebug, "debug", "d", false, "enable debug message logging")
	flag.StringVarP(&flagConfig, "config", "c", "", "specify custom ijvm configuration file")
	flag.StringVarP(&flagOutput, "output", "o", "", "specify output file. (default out.ijvm)")
	flag.BoolVarP(&flagForce, "force", "f", false, "ignore most error messages and just yolo through")
	flag.BoolVarP(&flagAutoWide, "widen", "w", false, "automatically add WIDE operations when required")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s inputfile\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	log = logging.MustGetLogger("main")
	format := logging.MustStringFormatter(`%{module:10.10s} [%{color}%{level:.4s}%{color:reset}] %{message}`)
	logging.SetFormatter(format)
	logging.SetBackend(logging.NewLogBackend(os.Stderr, "", 0))
	if flagDebug {
		logging.SetLevel(logging.DEBUG, "")
	} else if flagInfo {
		logging.SetLevel(logging.INFO, "")
	} else {
		logging.SetLevel(logging.NOTICE, "")
	}
}


func main() {
	args := flag.Args()
	if len(args) == 0 {
		log.Critical("Please specify a file to compile")
		os.Exit(1)
	}

	var config *opconf.OpConfig
	if flagConfig == "" {
		config = opconf.NewDefaultOpConfig()
	} else {
		config = opconf.NewOpConfigFromPath(flagConfig)
	}

	input := args[0]
	output := flagOutput
	if output == "" {
		output = "out.ijvm"
	}

	asm := ijvmasm.NewAssembler(input, config)
	asm.AutoWide = flagAutoWide
	ok, err := asm.Parse()

	if err != nil && !flagForce {
		log.Critical("Assembly prematurely aborted:")
		log.Critical(err.Error())
		os.Exit(1)
	}

	if !ok && !flagForce {
		log.Critical("Assembly failed")
		os.Exit(1)
	}

	var out io.Writer = os.Stdout
	if output != "-" {
		outf, err := os.Create(output)
		if err != nil {
			log.Critical("Could not open outputFile file:")
			log.Critical(err.Error())
			os.Exit(1)
		}
		defer outf.Close()
		out = outf
	}

	err = asm.Generate(out)
	if err != nil {
		log.Error(err.Error())
	}
}
