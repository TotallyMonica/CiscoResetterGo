package templates

var Client = ``

var JobApi = ``

var Device = `{{define "title"}}Configure device resetting parameters{{end}}
{{define "body"}}
<script>
    function getLabel(reqLabel) {
        let labels = document.getElementsByTagName("label");
        for (let i = 0; i < labels.length; i++) {
            if (labels[i].getAttribute("for") == reqLabel) {
                return i;
            }
        }
        return -1;
    }
    function toggleDefaultsRequired() {
        let defaultsCheckbox = document.getElementById("defaults");
        let defaultsFile = document.getElementById("defaultsFile");
        let defaultsLabel = document.getElementsByTagName("label")[getLabel("defaults")];
        defaultsFile.required = defaultsCheckbox.required;
        if (defaultsCheckbox.checked) {
            defaultsFile.style.display = '';
            defaultsLabel.style.display = '';
        } else {
            defaultsFile.style.display = 'none';
            defaultsLabel.style.display = 'none';
        }
        console.log("Defaults file is now " + ((document.getElementById("defaultsFile").required) ? "" : "no longer ") + "required");
    }
    function toggleBackupExtras() {
        let backupCheckbox = document.getElementById("backup");
        let builtinInput = document.getElementById("builtin");
        let dhcpInput = document.getElementById("dhcp");
        let destinationInput = document.getElementById("destination");
        let dhcpLabel = document.getElementsByTagName("label")[getLabel("dhcp")];
        let builtinLabel = document.getElementsByTagName("label")[getLabel("builtin")];
        let destinationLabel = document.getElementsByTagName("label")[getLabel("destination")];;
        if (backupCheckbox.checked) {
            dhcpInput.style.display = '';
            dhcpLabel.style.display = '';
            builtinInput.style.display = '';
            builtinLabel.style.display = '';
            destinationInput.style.display = '';
            destinationLabel.style.display = '';
            destinationInput.required = true    ;
        } else {
            dhcpInput.style.display = 'none';
            dhcpLabel.style.display = 'none';
            builtinInput.style.display = 'none';
            builtinLabel.style.display = 'none';
            destinationInput.style.display = 'none';
            destinationLabel.style.display = 'none';
            destinationInput.required = false;
        }
        toggleTemporarySourceIpRequired()
    }
    function toggleTemporarySourceIpRequired() {
        let dhcpInput = document.getElementById("dhcp");
        let sourceInput = document.getElementById("source");
        let maskInput = document.getElementById("mask");
        let sourceLabel = document.getElementsByTagName("label")[getLabel("source")];
        let maskLabel = document.getElementsByTagName("label")[getLabel("mask")];

        sourceInput.required = !dhcpInput.checked;
        console.log("Source IP is now " + ((sourceInput.required) ? "" : "no longer ") + "required");
        if (!dhcpInput.checked) {
            sourceInput.style.display = '';
            sourceLabel.style.display = '';
        } else {
            sourceInput.style.display = 'none';
            sourceLabel.style.display = 'none';
        }
        maskInput.required = !dhcpInput.checked;
        console.log("Subnet mask is now " + ((!dhcpInput.checked) ? "" : "no longer ") + "required");
        if (!dhcpInput.checked) {
            maskLabel.style.display = '';
            maskInput.style.display = '';
        } else {
            maskLabel.style.display = 'none';
            maskInput.style.display = 'none';
        }
    }
    setTimeout(function() {
        let defaultsFile = document.getElementById("defaultsFile");
        let defaultsLabel = document.getElementsByTagName("label")[5];
        let defaultsCheckbox = document.getElementById("defaults");
        let dhcpCheckbox = document.getElementById("dhcp");
        let backupCheckbox = document.getElementById("backup");
        defaultsFile.style.display = 'none';
        defaultsLabel.style.display = 'none';
        defaultsCheckbox.addEventListener("change", toggleDefaultsRequired);
        backupCheckbox.addEventListener("change", toggleBackupExtras);
        dhcpCheckbox.addEventListener("change", toggleTemporarySourceIpRequired);
        dhcpCheckbox.checked = true;
        for (let i = 0; i < 2; i++) {
            toggleBackupExtras()
        }
    }, 10);

</script>
<form action='/reset/' method='post' enctype='multipart/form-data'>
    <br>
    <h6>Choose device type</h6>
    <div class=form-check>
        <label class='form-check-label' for='router'>Router</label>
        <input class='form-check-input' type='radio' name='device' id='router' value='router' required>
    </div>
    <div class=form-check>
        <label class='form-check-label' for='switch'>Switch</label>
        <input class='form-check-input' type='radio' name='device' id='switch' value='switch' required>
    </div>
    <br>

    <h6>Verbosity</h6>
    <div class=form-check>
        <label class='form-check-label' for='verbose'>Verbose? </label>
        <input class='form-check-input' type='checkbox' id='verbose' name='verbose' value='verbose'>
    </div>

    <br>
    <h6>Functionality</h6>
    <div class=form-check>
        <label class='form-check-label' for='reset'>Reset? </label>
        <input class='form-check-input' type='checkbox' id='reset' name='reset' value='reset'>
    </div>

    <div class=form-check>
        <label class='form-check-label' for='defaults'>Apply defaults? </label>
        <input class='form-check-input' type='checkbox' id='defaults' name='defaults' value='defaults'>
    </div>
    <div class=form-group>
        <label for='defaultsFile'>Defaults File</label>
        <input type='file' class='form-control-file' id='defaultsFile' name='defaultsFile'>
    </div>

    <br>
    <h6>Backups</h6>
    <div class=form-check>
        <label class='form-check-label' for='backup'>Backup config?</label>
        <input class='form-check-input' type='checkbox' id='backup' name='backup' value='backup'>
    </div>

    <div class=form-check>
        <label class='form-check-label' for='dhcp'>Use DHCP Address?</label>
        <input class='form-check-input' type='checkbox' id='dhcp' name='dhcp' value='dhcp'>
    </div>

    <div class=form-check>
        <label class='form-check-label' for='builtin'>Use built-in TFTP server?</label>
        <input class='form-check-input' type='checkbox' id='builtin' name='builtin' value='builtin'>
    </div>

    <br>
    <div class="form-group form-check-inline">
        <label class='form-check-label' for='source'>Temporary IP address for device</label>
        <input type="text" class="form-control" id="source" name="source" required>
    </div>

    <div class="form-group form-check-inline">
        <label class='form-check-label' for='mask'>Temporary subnet mask for device</label>
        <input type="text" class="form-control" id="mask" name="mask" required>
    </div>

    <br>
    <br>
    <div class="form-group form-check-inline">
        <label class='form-check-label' for='destination'>TFTP Server address</label>
        <input type="text" class="form-control" id="destination" name="destination">
    </div>

    <input type="hidden" id="port" name="port" value="{{ .Port }}">
    <input type="hidden" id="baud" name="baud" value="{{ .BaudRate }}">
    <input type="hidden" id="data" name="data" value="{{ .DataBits }}">
    <input type="hidden" id="parity" name="parity" value="{{ .Parity }}">
    <input type="hidden" id="stop" name="stop" value="{{ .StopBits }}">
    <br>
    <br>
    <input type="submit" value="Submit" class="btn btn-primary">
</form>
{{end}}
`

var Index = `{{define "title"}}
Cisco Resetter Go Homepage
{{end}}

{{define "body"}}
<br>
{{$jobs := .Jobs | len -}}
<h3>Jobs running: {{ .Jobs | len -}}</h3>
{{ if ne $jobs 0 }}
<table class="table table-hover">
    <tr>
        <th>Job Number</th>
        <th>Port</th>
        <th>Port configuration</th>
        <th>Device type</th>
        <th>Verbose output</th>
        <th>Reset Device</th>
        <th>Apply Defaults</th>
        <th>Defaults file</th>
        <th>Initiator</th>
    </tr>
    {{ range .Jobs }}
    <tr>
        <td><a href="/jobs/{{ .Number }}/?lines=30">{{ .Number }}</a></td>
        <td>{{ .Params.PortConfig.Port }}</td>
        <td>{{ .Params.PortConfig.ShortHand }}</td>
        <td>{{ .Params.DeviceType }}</td>
        <td>{{ if .Params.Verbose }}Yes{{ else }}No{{end}}</td>
        <td>{{ if .Params.Reset }}Yes{{ else }}No{{ end }}</td>
        <td>{{ if .Params.Defaults }}Yes{{ else }}No{{ end }}</td>
        <td>{{ if .Params.DefaultsFile }}{{ .Params.DefaultsFile }}{{ else }}N/A{{ end }}</td>
    <td>{{ .Initiator }}</td>
    </tr>
    {{ end }}
</table>
{{ end }}
<br>
{{$serial := .SerialPorts | len -}}
<h3>Serial ports present: {{ .SerialPorts | len -}}</h3>
{{if ne $serial 0}}
<table class="table table-hover">
    <tr>
        <th>Port</th>
        <th>Description</th>
        <th>USB?</th>
        <th>PID:VID</th>
        <th>Serial</th>
    </tr>
    {{ range .SerialPorts }}
    <tr>
        <td>{{ .Name }}</td>
        <td>{{ .Product }}</td>
        <td>{{ if .IsUSB }}Yes{{ else }}No{{ end }}</td>
        <td>{{ if .IsUSB }}{{ .PID }}:{{ .VID }}{{ end}}</td>
        <td>{{ if .IsUSB }}{{ .SerialNumber }}{{ end }}</td>
    </tr>
    {{ end }}
</table>
{{ end }}
{{end}}`
var Job = `{{ define "title" }}
Job {{ .Number }}
{{ end }}

{{ define "body" }}
    <meta http-equiv="refresh" content="5">
<p>Serial port: {{ .Params.PortConfig.Port }}</p>
<br>
<p>Output:</p>
<pre>
{{ .Output }}
</pre>
{{ end }}`
var Jobs = `{{define "title"}}
Jobs status
{{end}}
{{define "body"}}
{{ $jobs := . | len -}}
<h3>Jobs running: {{ . | len -}}</h3>
{{ if ne $jobs 0 }}
<table class="table table-hover">
    <tr>
        <th>Job Number</th>
        <th>Port</th>
        <th>Port configuration</th>
        <th>Device type</th>
        <th>Verbose output</th>
        <th>Reset Device</th>
        <th>Apply Defaults</th>
        <th>Defaults file</th>
        <th>Initiator</th>
        <th>Status</th>
    </tr>
    {{ range . }}
    <tr>
        <td><a href="/jobs/{{ .Number }}/?lines=30">{{ .Number }}</a></td>
        <td>{{ .Params.PortConfig.Port }}</td>
        <td>{{ .Params.PortConfig.ShortHand }}</td>
        <td>{{ .Params.DeviceType }}</td>
        <td>{{ if .Params.Verbose }}Yes{{ else }}No{{end}}</td>
        <td>{{ if .Params.Reset }}Yes{{ else }}No{{ end }}</td>
        <td>{{ if .Params.Defaults }}Yes{{ else }}No{{ end }}</td>
        <td>{{ if .Params.DefaultsFile }}{{ .Params.DefaultsFile }}{{ else }}N/A{{ end }}</td>
        <td>{{ .Initiator }}</td>
        <td>{{ .Status }}</td>
    </tr>
    {{ end }}
</table>
{{ end }}
{{ end }}`
var Layout = `{{define "layout"}}
<!DOCTYPE html>
<html lang="en" data-bs-theme="dark">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-QWTKZyjpPEjISv5WaRU9OFeRpok6YctnYmDr5pNlyT2bRjXh0JMhjY6hW+ALEwIH" crossorigin="anonymous">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <nav class="navbar navbar-expand-lg bg-body-tertiary p-2">
        <a class="navbar-brand" href="/">Cisco Resetter Go</a>
        <button class="navbar-toggler" type="button" data-toggle="collapse" data-target="#navbarSupportedContent" aria-controls="navbarSupportedContent" aria-expanded="false" aria-label="Toggle navigation">
            <span class="navbar-toggler-icon"></span>
        </button>

        <div class="collapse navbar-collapse" id="navbarText">
            <ul class="navbar-nav me-auto mb-2 mb-lg-0">
                <li class="nav-item active">
                    <a class="nav-link" aria-current="page" href="/">Home</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="/port/">New Job</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="/list/jobs/">Running Jobs</a>
                </li>
                <li class="nav-item">
                    <a class="nav-link" href="/list/ports/">Available ports</a>
                </li>
            </ul>
        </div>
    </nav>
    <div>
        <h1 class="p-2">
            {{template "title" .}}
        </h1>
    </div>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-QWTKZyjpPEjISv5WaRU9OFeRpok6YctnYmDr5pNlyT2bRjXh0JMhjY6hW+ALEwIH" crossorigin="anonymous">
    <div class="p-2">
        {{template "body" .}}
    </div>
</html>
{{end}}
`
var Port = `{{define "title"}}
Configure serial port
{{end}}

{{define "body"}}
<form action="/device/" method="post">
    <div class="form-group">
        <label for="device">
            Specify a serial device:
        </label>
        <select name='device' id='device' class='form-control'>
            {{ range . }}<option value="{{ .Name }}">{{ .Name }}{{ if .IsUSB }} (USB) {{ end }}</option>{{ end }}
        </select>
    </div>
    <br>
    <div class='form-group'>
        <label for='baud'>Baud rate: </label>
        <input type='number' id='baud' name='baud' class='form-control' value='9600'>
    </div>
    <br>
    <div class='form-group'>
        <label for='data'>Data bits: </label>
        <input type='number' id='data' name='data' class='form-control' min='5' max='8' value='8'>
    </div>
    <br>
    <div class='form-group'>
        <label for='parity'>Parity: </label>
        <select name='parity' id='parity' class='form-control'>
            <option value='no'>No parity</option>
            <option value='even'>Even parity</option>
            <option value='odd'>Odd parity</option>
            <option value='space'>Space parity</option>
            <option value='mark'>Mark parity</option>
        </select>
    </div>
    <br>
    <div class='form-group'>
        <label for='stop'>Stop bits: </label>
        <br>
        <select name='stop' id='stop' class='form-control'>
            <option value='one'>1 stop bit</option>
            <option value='opf'>1.5 stop bits</option>
            <option value='two'>2 stop bits</option>
        </select>
    </div>
    <br>
    <input type="submit" value="Submit" class="btn btn-primary">
</form>
{{end}}`
var Ports = `{{define "title"}}
Serial ports list
{{end}}
{{define "body"}}
{{$serial := . | len -}}
<h3>Serial ports present: {{ . | len -}}</h3>
{{if ne $serial 0}}
<table class="table table-hover">
    <tr>
        <th>Port</th>
        <th>Description</th>
        <th>USB?</th>
        <th>PID:VID</th>
        <th>Serial</th>
    </tr>
    {{ range . }}
    <tr>
        <td>{{ .Name }}</td>
        <td>{{ .Product }}</td>
        <td>{{ if .IsUSB }}Yes{{ else }}No{{ end }}</td>
        <td>{{ if .IsUSB }}{{ .PID }}:{{ .VID }}{{ end}}</td>
        <td>{{ if .IsUSB }}{{ .SerialNumber }}{{ end }}</td>
    </tr>
    {{ end }}
</table>
{{ end }}
{{end}}`
var Reset = `{{define "title"}}Chosen reset parameters{{end}}
{{define "body"}}
<p>Job number {{ .Number }} queued with the following settings:</p>
<ul>
    <li>Serial port: {{ .Params.PortConfig.Port }}</li>
    <li>Baud rate: {{ .Params.PortConfig.BaudRate }}</li>
    <li>Data bits: {{ .Params.PortConfig.DataBits }}</li>
    <li>Parity: {{ .Params.PortConfig.Parity }}</li>
    <li>Stop bits: {{ .Params.PortConfig.StopBits }}</li>
    <li>Device type: {{ .Params.DeviceType }} </li>
    {{ if .Params.Verbose }}<li>Have verbose output</li>{{ end }}
    {{ if .Params.Reset }}<li>Reset device</li>{{ end }}
    {{ if .Params.Defaults }}<li>Apply default settings</li>{{ end }}
    {{ if .Params.DefaultsFile}}<li>Defaults file: {{ .Params.DefaultsFile }}</li>{{ end }}
    {{ if .Params.DefaultsContents}}<li>Defaults file contents: {{ .Params.DefaultsContents}}</li>{{ end }}
</ul>
{{end}}
`
