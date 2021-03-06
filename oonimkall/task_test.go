package oonimkall_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/ooni/probe-engine/model"
	"github.com/ooni/probe-engine/oonimkall"
)

type eventlike struct {
	Key   string                 `json:"key"`
	Value map[string]interface{} `json:"value"`
}

func TestIntegrationGood(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"log_level": "DEBUG",
		"name": "Example",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	// interrupt the task so we also exercise this functionality
	go func() {
		<-time.After(time.Second)
		task.Interrupt()
	}()
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
	}
	// make sure we only see task_terminated at this point
	for {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		if event.Key != "task_terminated" {
			t.Fatalf("unexpected event.Key: %s", event.Key)
		}
		break
	}
}

func TestIntegrationGoodWithoutGeoIPLookup(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"log_level": "DEBUG",
		"name": "Example",
		"options": {
			"no_geoip": true,
			"no_resolver_lookup": true,
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntegrationWithMeasurementFailure(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"log_level": "DEBUG",
		"name": "ExampleWithFailure",
		"options": {
			"no_geoip": true,
			"no_resolver_lookup": true,
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntegrationInvalidJSON(t *testing.T) {
	task, err := oonimkall.StartTask(`{`)
	var syntaxerr *json.SyntaxError
	if !errors.As(err, &syntaxerr) {
		t.Fatal("not the expected error")
	}
	if task != nil {
		t.Fatal("task is not nil")
	}
}

func TestIntegrationUnsupportedSetting(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"input_filepaths": ["/nonexistent"],
		"log_level": "DEBUG",
		"name": "Example",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		if event.Key == "failure.startup" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("did not see failure.startup")
	}
}

func TestIntegrationEmptyStateDir(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"log_level": "DEBUG",
		"name": "Example",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		if event.Key == "failure.startup" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("did not see failure.startup")
	}
}

func TestIntegrationEmptyAssetsDir(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"log_level": "DEBUG",
		"name": "Example",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		if event.Key == "failure.startup" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("did not see failure.startup")
	}
}

func TestIntegrationUnknownExperiment(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"log_level": "DEBUG",
		"name": "Antani",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		if event.Key == "failure.startup" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("did not see failure.startup")
	}
}

func TestIntegrationInconsistentGeoIPSettings(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"log_level": "DEBUG",
		"name": "Example",
		"options": {
			"no_geoip": true,
			"no_resolver_lookup": false,
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		if event.Key == "failure.startup" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("did not see failure.startup")
	}
}

func TestIntegrationInputIsRequired(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"log_level": "DEBUG",
		"name": "ExampleWithInput",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		if event.Key == "failure.startup" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("did not see failure.startup")
	}
}

func TestIntegrationMaxRuntime(t *testing.T) {
	begin := time.Now()
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"inputs": ["a", "b", "c"],
		"name": "ExampleWithInput",
		"options": {
			"max_runtime": 1,
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	for !task.IsDone() {
		task.WaitForNextEvent()
	}
	// The runtime is long because of ancillary operations and is even more
	// longer because of self shaping we may be performing (especially in
	// CI builds) using `-tags shaping`). We have experimentally determined
	// that ~10 seconds is the typical CI test run time. See:
	//
	// 1. https://github.com/ooni/probe-engine/pull/588/checks?check_run_id=667263788
	//
	// 2. https://github.com/ooni/probe-engine/pull/588/checks?check_run_id=667263855
	if time.Now().Sub(begin) > 10*time.Second {
		t.Fatal("expected shorter runtime")
	}
}

func TestIntegrationInterruptExampleWithInput(t *testing.T) {
	// We cannot use WebConnectivity until it's written in Go since
	// measurement-kit may not always be available
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"inputs": [
			"http://www.kernel.org/",
			"http://www.x.org/",
			"http://www.microsoft.com/",
			"http://www.slashdot.org/",
			"http://www.repubblica.it/",
			"http://www.google.it/",
			"http://ooni.org/"
		],
		"name": "ExampleWithInputNonInterruptible",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		switch event.Key {
		case "status.measurement_start":
			go task.Interrupt()
		}
		// We compress the keys. What matters is basically that we
		// see just one of the many possible measurements here.
		if keys == nil || keys[len(keys)-1] != event.Key {
			keys = append(keys, event.Key)
		}
	}
	expect := []string{
		"status.queued",
		"status.started",
		"status.progress",
		"status.geoip_lookup",
		"status.resolver_lookup",
		"status.progress",
		"status.report_create",
		"status.measurement_start",
		"log",
		"status.progress",
		"measurement",
		"status.measurement_submission",
		"status.measurement_done",
		"status.end",
		"task_terminated",
	}
	if diff := cmp.Diff(expect, keys); diff != "" {
		t.Fatal(diff)
	}
}

func TestIntegrationInterruptNdt7(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"name": "Ndt7",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		<-time.After(11 * time.Second)
		task.Interrupt()
	}()
	var keys []string
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		// We compress the keys because we don't know how many
		// status.progress we will see. What matters is that we
		// don't see a measurement submission, since it means
		// that we have interrupted the measurement.
		if keys == nil || keys[len(keys)-1] != event.Key {
			keys = append(keys, event.Key)
		}
	}
	expect := []string{
		"status.queued",
		"status.started",
		"status.progress",
		"status.geoip_lookup",
		"status.resolver_lookup",
		"status.progress",
		"status.report_create",
		"status.measurement_start",
		"status.progress",
		"status.end",
		"task_terminated",
	}
	if diff := cmp.Diff(expect, keys); diff != "" {
		t.Fatal(diff)
	}
}

func TestIntegrationCountBytesForExample(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"name": "Example",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var downloadKB, uploadKB float64
	for !task.IsDone() {
		eventstr := task.WaitForNextEvent()
		var event eventlike
		if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
			t.Fatal(err)
		}
		switch event.Key {
		case "status.end":
			downloadKB = event.Value["downloaded_kb"].(float64)
			uploadKB = event.Value["uploaded_kb"].(float64)
		}
	}
	if downloadKB == 0 {
		t.Fatal("downloadKB is zero")
	}
	if uploadKB == 0 {
		t.Fatal("uploadKB is zero")
	}
}

func TestIntegrationPrivacySettings(t *testing.T) {
	do := func(saveASN, saveCC, saveIP bool) (string, string, string) {
		task, err := oonimkall.StartTask(fmt.Sprintf(`{
			"assets_dir": "../testdata/oonimkall/assets",
			"name": "Example",
			"options": {
				"save_real_probe_asn": %+v,
				"save_real_probe_cc": %+v,
				"save_real_probe_ip": %+v,
				"software_name": "oonimkall-test",
				"software_version": "0.1.0"
			},
			"state_dir": "../testdata/oonimkall/state"
		}`, saveASN, saveCC, saveIP))
		if err != nil {
			t.Fatal(err)
		}
		var measurement *model.Measurement
		for !task.IsDone() {
			eventstr := task.WaitForNextEvent()
			var event eventlike
			if err := json.Unmarshal([]byte(eventstr), &event); err != nil {
				t.Fatal(err)
			}
			switch event.Key {
			case "measurement":
				v := []byte(event.Value["json_str"].(string))
				measurement = new(model.Measurement)
				if err := json.Unmarshal(v, &measurement); err != nil {
					t.Fatal(err)
				}
			}
		}
		if measurement == nil {
			t.Fatal("measurement is nil")
		}
		return measurement.ProbeASN, measurement.ProbeCC, measurement.ProbeIP
	}
	asn, cc, ip := do(false, false, false)
	if asn != "AS0" || cc != "ZZ" || ip != "127.0.0.1" {
		t.Fatal("unexpected result")
	}
	asn, cc, ip = do(false, false, true)
	if asn != "AS0" || cc != "ZZ" || ip == "127.0.0.1" {
		t.Fatalf("unexpected result: ip was supposed to be %+v and instead is 127.0.0.1", ip)
	}
	asn, cc, ip = do(false, true, false)
	if asn != "AS0" || cc == "ZZ" || ip != "127.0.0.1" {
		t.Fatalf("unexpected result: cc was supposed to be %+v and instead is ZZ", cc)
	}
	asn, cc, ip = do(true, false, false)
	if asn == "AS0" || cc != "ZZ" || ip != "127.0.0.1" {
		t.Fatalf("unexpected result: asn was supposed to be %+v and instead is AS0", asn)
	}
	asn, cc, ip = do(true, false, true)
	if asn == "AS0" || cc != "ZZ" || ip == "127.0.0.1" {
		t.Fatalf(`unexpected result: asn and ip were supposed to be %+v and %+v
			, respectively. instead are AS0 and/or 127.0.0.1`, asn, ip)
	}
	asn, cc, ip = do(true, true, false)
	if asn == "AS0" || cc == "ZZ" || ip != "127.0.0.1" {
		t.Fatalf(`unexpected result: asn and cc were supposed to be %+v and %+v
			, respectively. instead are AS0 and/or ZZ`, asn, cc)
	}
	asn, cc, ip = do(false, true, true)
	if asn != "AS0" || cc == "ZZ" || ip == "127.0.0.1" {
		t.Fatalf(`unexpected result: cc and ip were supposed to be %+v and %+v
			, respectively. instead are ZZ and/or 127.0.0.1`, cc, ip)
	}
	asn, cc, ip = do(true, true, true)
	if asn == "AS0" || cc == "ZZ" || ip == "127.0.0.1" {
		t.Fatalf(`unexpected result: asn, cc and ip were supposed to be %+v, %+v and %+v
			, respectively. instead are AS0, ZZ and/or 127.0.0.1`, asn, cc, ip)
	}
}

func TestIntegrationNonblock(t *testing.T) {
	task, err := oonimkall.StartTask(`{
		"assets_dir": "../testdata/oonimkall/assets",
		"name": "Example",
		"options": {
			"software_name": "oonimkall-test",
			"software_version": "0.1.0"
		},
		"state_dir": "../testdata/oonimkall/state"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if !task.IsRunning() {
		t.Fatal("The runner should be running at this point")
	}
	// If the task blocks because it emits too much events, this test
	// will run forever and will be killed. Because we have room for up
	// to 128 events in the buffer, we should hopefully be fine.
	for task.IsRunning() {
		time.Sleep(time.Second)
	}
	for !task.IsDone() {
		task.WaitForNextEvent()
	}
}
