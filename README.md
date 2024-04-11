# Go Cisco Resetter
## A rewrite of [TotallyMonica/CiscoReset](https://github.com/TotallyMoinca/CiscoReset) in Go

## Why this?
After using the first version of this, I discovered that the lab that I work in will reset the computers after every reboot and are not able to connect to the main network. As such, reinstalling the dependencies to run the Python script was needlessly difficult.

While it is possible to compile Python scripts to a single executable, to me it made more sense to rewrite it in Go as that was a language that I was trying to learn.

## To-Do
- [ ] Ensure Cisco 4221 is properly reset
- [ ] Ensure Cisco 2960 series is properly reset
- [ ] Test Windows 7 compatibility
- [ ] Test Linux compatibility (Baseline: Ubuntu 16.04)
- [ ] Set custom defaults via JSON
- [ ] Flags for identifying what to configure
- [ ] Mail/push alerts upon completion
- [ ] Handle password recovery being disabled
- [ ] Back up configs prior to reset