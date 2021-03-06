// Package libminiooni implements the cmd/miniooni CLI. Miniooni is our
// experimental client used for research and QA testing.
//
// This CLI has CLI options that do not conflict with Measurement Kit
// v0.10.x CLI options. There are some options conflict with the legacy
// OONI Probe CLI options. Perfect backwards compatibility is not a
// design goal for miniooni. Rather, we aim to have as little conflict
// as possible such that we can run side by side QA checks.
//
// We extracted this package from cmd/miniooni to allow us to further
// integrate the miniooni CLI into other binaries (see for example the
// code at github.com/bassosimone/aladdin). In retrospect, this isn't
// particularly simple to keep up to date because it is complex to sync
// the dependencies used by Psiphon, which need precise pinning.
package libminiooni

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/apex/log"
	engine "github.com/ooni/probe-engine"
	"github.com/ooni/probe-engine/internal/humanizex"
	"github.com/ooni/probe-engine/model"
	"github.com/ooni/probe-engine/netx/selfcensor"
	"github.com/pborman/getopt/v2"
)

// Options contains the options you can set from the CLI.
type Options struct {
	Annotations      []string
	ExtraOptions     []string
	HomeDir          string
	Inputs           []string
	InputFilePath    string
	NoBouncer        bool
	NoGeoIP          bool
	NoJSON           bool
	NoCollector      bool
	ProbeServicesURL string
	Proxy            string
	ReportFile       string
	SelfCensorSpec   string
	TorArgs          []string
	TorBinary        string
	Tunnel           string
	Verbose          bool
}

const (
	softwareName    = "miniooni"
	softwareVersion = engine.Version
)

var (
	globalOptions Options
	startTime     = time.Now()
)

func init() {
	getopt.FlagLong(
		&globalOptions.Annotations, "annotation", 'A', "Add annotaton", "KEY=VALUE",
	)
	getopt.FlagLong(
		&globalOptions.ExtraOptions, "option", 'O',
		"Pass an option to the experiment", "KEY=VALUE",
	)
	getopt.FlagLong(
		&globalOptions.InputFilePath, "file", 'f',
		"Path to input file to supply test-dependent input. File must contain one input per line.", "PATH",
	)
	getopt.FlagLong(
		&globalOptions.HomeDir, "home", 0,
		"Force specific home directory", "PATH",
	)
	getopt.FlagLong(
		&globalOptions.Inputs, "input", 'i',
		"Add test-dependent input to the test input", "INPUT",
	)
	getopt.FlagLong(
		&globalOptions.NoBouncer, "no-bouncer", 0, "Don't use the OONI bouncer",
	)
	getopt.FlagLong(
		&globalOptions.NoGeoIP, "no-geoip", 'g',
		"Disable including ASN information into the report",
	)
	getopt.FlagLong(
		&globalOptions.NoJSON, "no-json", 'N', "Disable writing to disk",
	)
	getopt.FlagLong(
		&globalOptions.NoCollector, "no-collector", 'n', "Don't use a collector",
	)
	getopt.FlagLong(
		&globalOptions.ProbeServicesURL, "probe-services", 0,
		"Set the URL of the probe-services instance you want to use", "URL",
	)
	getopt.FlagLong(
		&globalOptions.Proxy, "proxy", 0, "Set the proxy URL", "URL",
	)
	getopt.FlagLong(
		&globalOptions.ReportFile, "reportfile", 'o',
		"Set the report file path", "PATH",
	)
	getopt.FlagLong(
		&globalOptions.SelfCensorSpec, "self-censor-spec", 0,
		"Enable and configure self censorship", "JSON",
	)
	getopt.FlagLong(
		&globalOptions.TorArgs, "tor-args", 0,
		"Extra args for tor binary (may be specified multiple times)",
	)
	getopt.FlagLong(
		&globalOptions.TorBinary, "tor-binary", 0,
		"Specify path to a specific tor binary",
	)
	getopt.FlagLong(
		&globalOptions.Tunnel, "tunnel", 0,
		"Name of the tunnel to use (one of `tor`, `psiphon`)",
	)
	getopt.FlagLong(
		&globalOptions.Verbose, "verbose", 'v', "Increase verbosity",
	)
}

func fatalWithString(msg string) {
	panic(msg)
}

func fatalIfFalse(cond bool, msg string) {
	if !cond {
		log.Warn(msg)
		panic(msg)
	}
}

// Main is the main function of miniooni. This function parses the command line
// options and uses a global state. Use MainWithConfiguration if you want to avoid
// using any global state and relying on command line options.
//
// This function will panic in case of a fatal error. It is up to you that
// integrate this function to either handle the panic of ignore it.
func Main() {
	getopt.Parse()
	fatalIfFalse(len(getopt.Args()) == 1, "Missing experiment name")
	MainWithConfiguration(getopt.Arg(0), globalOptions)
}

func split(s string) (string, string, error) {
	v := strings.SplitN(s, "=", 2)
	if len(v) != 2 {
		return "", "", errors.New("invalid key-value pair")
	}
	return v[0], v[1], nil
}

func fatalOnError(err error, msg string) {
	if err != nil {
		log.WithError(err).Warn(msg)
		panic(msg)
	}
}

func warnOnError(err error, msg string) {
	if err != nil {
		log.WithError(err).Warn(msg)
	}
}

func mustMakeMap(input []string) (output map[string]string) {
	output = make(map[string]string)
	for _, opt := range input {
		key, value, err := split(opt)
		fatalOnError(err, "cannot split key-value pair")
		output[key] = value
	}
	return
}

func mustParseURL(URL string) *url.URL {
	rv, err := url.Parse(URL)
	fatalOnError(err, "cannot parse URL")
	return rv
}

type logHandler struct {
	io.Writer
}

func (h *logHandler) HandleLog(e *log.Entry) (err error) {
	s := fmt.Sprintf("[%14.6f] <%s> %s", time.Since(startTime).Seconds(), e.Level, e.Message)
	if len(e.Fields) > 0 {
		s += fmt.Sprintf(": %+v", e.Fields)
	}
	s += "\n"
	_, err = h.Writer.Write([]byte(s))
	return
}

// See https://gist.github.com/miguelmota/f30a04a6d64bd52d7ab59ea8d95e54da
func gethomedir(optionsHome string) string {
	if optionsHome != "" {
		return optionsHome
	}
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	if runtime.GOOS == "linux" {
		home := os.Getenv("XDG_CONFIG_HOME")
		if home != "" {
			return home
		}
		// fallthrough
	}
	return os.Getenv("HOME")
}

func loadFileInputs(opts *Options) {
	if len(opts.InputFilePath) != 0 {
		if len(opts.Inputs) != 0 {
			fatalWithString("inputs can either be supplied through file or command line, but not both")
		}
		content, err := ioutil.ReadFile(opts.InputFilePath)
		fatalOnError(err, "cannot read input file")
		// Implementation note: when you save file with vim, you have newline at
		// end of file and you don't want to consider that an input line. While there
		// ignore any other empty line that may occur inside the file.
		for _, input := range strings.Split(string(content), "\n") {
			if input != "" {
				opts.Inputs = append(opts.Inputs, input)
			}
		}
	}
}

// MainWithConfiguration is the miniooni main with a specific configuration
// represented by the experiment name and the current options.
//
// This function will panic in case of a fatal error. It is up to you that
// integrate this function to either handle the panic of ignore it.
func MainWithConfiguration(experimentName string, currentOptions Options) {
	extraOptions := mustMakeMap(currentOptions.ExtraOptions)
	annotations := mustMakeMap(currentOptions.Annotations)

	err := selfcensor.MaybeEnable(currentOptions.SelfCensorSpec)
	fatalOnError(err, "cannot parse --self-censor-spec argument")

	logger := &log.Logger{Level: log.InfoLevel, Handler: &logHandler{Writer: os.Stderr}}
	if currentOptions.Verbose {
		logger.Level = log.DebugLevel
	}
	if currentOptions.ReportFile == "" {
		currentOptions.ReportFile = "report.jsonl"
	}
	log.Log = logger

	homeDir := gethomedir(currentOptions.HomeDir)
	fatalIfFalse(homeDir != "", "home directory is empty")
	miniooniDir := path.Join(homeDir, ".miniooni")
	assetsDir := path.Join(miniooniDir, "assets")
	err = os.MkdirAll(assetsDir, 0700)
	fatalOnError(err, "cannot create assets directory")
	log.Infof("miniooni state directory: %s", miniooniDir)

	var proxyURL *url.URL
	if currentOptions.Proxy != "" {
		proxyURL = mustParseURL(currentOptions.Proxy)
	}

	kvstore2dir := filepath.Join(miniooniDir, "kvstore2")
	kvstore, err := engine.NewFileSystemKVStore(kvstore2dir)
	fatalOnError(err, "cannot create kvstore2 directory")

	config := engine.SessionConfig{
		AssetsDir: assetsDir,
		KVStore:   kvstore,
		Logger:    logger,
		PrivacySettings: model.PrivacySettings{
			// See https://github.com/ooni/explorer/issues/495#issuecomment-704101604
			IncludeASN:     currentOptions.NoGeoIP == false,
			IncludeCountry: true,
		},
		ProxyURL:        proxyURL,
		SoftwareName:    softwareName,
		SoftwareVersion: softwareVersion,
		TorArgs:         currentOptions.TorArgs,
		TorBinary:       currentOptions.TorBinary,
	}
	if currentOptions.ProbeServicesURL != "" {
		config.AvailableProbeServices = []model.Service{{
			Address: currentOptions.ProbeServicesURL,
			Type:    "https",
		}}
	}

	sess, err := engine.NewSession(config)
	fatalOnError(err, "cannot create measurement session")
	defer func() {
		sess.Close()
		log.Infof("whole session: recv %s, sent %s",
			humanizex.SI(sess.KibiBytesReceived()*1024, "byte"),
			humanizex.SI(sess.KibiBytesSent()*1024, "byte"),
		)
	}()
	log.Infof("miniooni temporary directory: %s", sess.TempDir())

	err = sess.MaybeStartTunnel(context.Background(), currentOptions.Tunnel)
	fatalOnError(err, "cannot start session tunnel")

	if !currentOptions.NoBouncer {
		log.Info("Looking up OONI backends; please be patient...")
		err := sess.MaybeLookupBackends()
		fatalOnError(err, "cannot lookup OONI backends")
	}
	log.Info("Looking up your location; please be patient...")
	err = sess.MaybeLookupLocation()
	fatalOnError(err, "cannot lookup your location")
	log.Infof("- IP: %s", sess.ProbeIP())
	log.Infof("- country: %s", sess.ProbeCC())
	log.Infof("- network: %s (%s)", sess.ProbeNetworkName(), sess.ProbeASNString())
	log.Infof("- resolver's IP: %s", sess.ResolverIP())
	log.Infof("- resolver's network: %s (%s)", sess.ResolverNetworkName(),
		sess.ResolverASNString())

	builder, err := sess.NewExperimentBuilder(experimentName)
	fatalOnError(err, "cannot create experiment builder")

	// load inputs from file, if present
	loadFileInputs(&currentOptions)

	if builder.InputPolicy() == engine.InputRequired {
		if len(currentOptions.Inputs) <= 0 {
			log.Info("Fetching test lists")
			client, err := sess.NewOrchestraClient(context.Background())
			fatalOnError(err, "cannot create new orchestra client")
			list, err := client.FetchURLList(context.Background(), model.URLListConfig{
				CountryCode: sess.ProbeCC(),
				Limit:       17,
			})
			fatalOnError(err, "cannot fetch test lists")
			for _, entry := range list {
				currentOptions.Inputs = append(currentOptions.Inputs, entry.URL)
			}
		}
	} else if builder.InputPolicy() == engine.InputOptional {
		if len(currentOptions.Inputs) == 0 {
			currentOptions.Inputs = append(currentOptions.Inputs, "")
		}
	} else if len(currentOptions.Inputs) != 0 {
		fatalWithString("this experiment does not expect any input")
	} else {
		// Tests that do not expect input internally require an empty input to run
		currentOptions.Inputs = append(currentOptions.Inputs, "")
	}
	intregexp := regexp.MustCompile("^[0-9]+$")
	for key, value := range extraOptions {
		if value == "true" || value == "false" {
			err := builder.SetOptionBool(key, value == "true")
			fatalOnError(err, "cannot set boolean option")
		} else if intregexp.MatchString(value) {
			number, err := strconv.ParseInt(value, 10, 64)
			fatalOnError(err, "cannot parse integer option")
			err = builder.SetOptionInt(key, number)
			fatalOnError(err, "cannot set integer option")
		} else {
			err := builder.SetOptionString(key, value)
			fatalOnError(err, "cannot set string option")
		}
	}
	experiment := builder.NewExperiment()
	defer func() {
		log.Infof("experiment: recv %s, sent %s",
			humanizex.SI(experiment.KibiBytesReceived()*1024, "byte"),
			humanizex.SI(experiment.KibiBytesSent()*1024, "byte"),
		)
	}()

	if !currentOptions.NoCollector {
		log.Info("Opening report; please be patient...")
		err := experiment.OpenReport()
		fatalOnError(err, "cannot open report")
		defer experiment.CloseReport()
		log.Infof("Report ID: %s", experiment.ReportID())
	}

	inputCount := len(currentOptions.Inputs)
	inputCounter := 0
	for _, input := range currentOptions.Inputs {
		inputCounter++
		if input != "" {
			log.Infof("[%d/%d] running with input: %s", inputCounter, inputCount, input)
		}
		measurement, err := experiment.Measure(input)
		warnOnError(err, "measurement failed")
		measurement.AddAnnotations(annotations)
		measurement.Options = currentOptions.ExtraOptions
		if !currentOptions.NoCollector {
			log.Infof("submitting measurement to OONI collector; please be patient...")
			err := experiment.SubmitAndUpdateMeasurement(measurement)
			warnOnError(err, "submitting measurement failed")
		}
		if !currentOptions.NoJSON {
			// Note: must be after submission because submission modifies
			// the measurement to include the report ID.
			log.Infof("saving measurement to disk")
			err := experiment.SaveMeasurement(measurement, currentOptions.ReportFile)
			warnOnError(err, "saving measurement failed")
		}
	}
}
