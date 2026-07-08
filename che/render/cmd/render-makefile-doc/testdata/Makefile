#[what] fixture makefile
FOO := bar

##[>] Parameters [genai-include]
#[what] mode with values
#[vals] delta|all
DRY_RUN ?=
#[what] plain param no vals
VERBOSE ?=
#[what] bare export, read from env
export EXPORTED
UNDOCUMENTED := internal
##[<] Parameters

##[>] Wrappers [genai-include]
#[what] do quick sync
run-sync-quick: run-host-thing run-repo-thing
##[<] Wrappers

##[>] Onto Host [genai-include]
#[what] place a thing on host
run-host-thing:
	@echo host

run-host-undocumented:
	@echo plain

#[what] this what is interrupted
#[why] the why comment resets pending what
run-host-why-only:
	@echo why
##[<] Onto Host

##[>] Internal
#[what] helper, must be excluded
run-helper:
	@echo helper
##[<] Internal

##[>] Onto Repo (CI) [genai-include]
#[what] outer repo target
run-repo-thing:
	@echo repo

###[>] VM
#[what] nested target
run-vm-nested:
	@echo nested
###[<] VM
##[<] Onto Repo (CI)
