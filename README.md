# Go Cisco Resetter
## A rewrite of [TotallyMonica/CiscoReset](https://github.com/TotallyMoinca/CiscoReset) in Go

## Compile instructions

### Dependencies
- Go >= 1.20.14

### Compile
```
git clone https://github.com/TotallyMonica/CiscoResetterGo
cd CiscoResetterGo
go build main
```

## Run Instructions

### Windows
```
./main.exe { --web-server | <--router [--router-defaults /path/to/router_defaults.json] | --switch --switch-defaults /path/to/switch_defaults.json]> [--skip-reset] } [--debug]
```

### Linux
```
./main { --web-server | <--router [--router-defaults /path/to/router_defaults.json] | --switch --switch-defaults /path/to/switch_defaults.json]> [--skip-reset] } [--debug]
```

## Why this?
After using the first version of this, I discovered that the lab that I work in will reset the computers after every reboot and are not able to connect to the main network. As such, reinstalling the dependencies to run the Python script was needlessly difficult.

While it is possible to compile Python scripts to a single executable, to me it made more sense to rewrite it in Go as that was a language that I was trying to learn.

## Tested with:
- Cisco 4221
- Cisco 2960G Series
- Cisco 2960 Plus Series
- Cisco 2960-C Series PoE

## To-Do
- [x] ~~Ensure Cisco 4221 is properly reset~~ Confirmed 4/11/2024
- [x] ~~Ensure Cisco 2960 series is properly reset~~ Confirmed 4/16/2024
- [x] ~~Test Windows 7 compatibility~~ Confirmed 4/25/2024
- [x] ~~Test Linux compatibility (Baseline: Ubuntu 16.04)~~ Confirmed 4/25/2024
- [x] ~~Set custom defaults via JSON~~ Switch functionality confirmed 5/18/2024
- [ ] Flags for identifying what to configure
- [ ] Mail/push alerts upon completion
- [ ] Handle password recovery being disabled
- [ ] Back up configs prior to reset
- [ ] Configure serial port via switches
- [x] ~~Allow changing of serial port settings (Currently only allowing 9600 8N1)~~ Written 4/25/2024
