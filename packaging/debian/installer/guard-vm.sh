#!/bin/sh
# Debian Installer entry guard. This does not authorize or select a target disk.
# DMI identifies an expected VM family; it cannot detect host disk passthrough.

locasos_guard_read_dmi() {
    [ -r "$1" ] || return 1
    tr -d '\000\r' < "$1" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//'
}

locasos_guard_check() {
    # A fixture root is accepted only by this sourced function for tests.
    # The executable entrypoint below always checks the real /sys.
    [ "$#" -eq 1 ] || return 1
    locasos_sys_root=$1
    if [ ! -d "$locasos_sys_root/firmware/efi" ]; then
        echo 'LocasOS installer requires a UEFI virtual machine.' >&2
        return 1
    fi
    locasos_vendor=$(locasos_guard_read_dmi "$locasos_sys_root/class/dmi/id/sys_vendor") || return 1
    locasos_product=$(locasos_guard_read_dmi "$locasos_sys_root/class/dmi/id/product_name") || return 1
    case "$locasos_vendor:$locasos_product" in
        'Microsoft Corporation:Virtual Machine'|\
        'QEMU:Standard PC (Q35 + ICH9, 2009)'|\
        'QEMU:Standard PC (i440FX + PIIX, 1996)')
            return 0
            ;;
        *)
            echo 'LocasOS installer supports only the documented Hyper-V or QEMU VM families.' >&2
            return 1
            ;;
    esac
}

locasos_guard_refuse() {
    # A failed preseed early_command can otherwise be acknowledged and skipped.
    # Stay here until the machine is reset; never return to the installer menu.
    trap '' HUP INT TERM
    echo 'LOCASOS_INSTALLER_REFUSED: VM/UEFI check failed. Reboot to leave this screen.' >&2
    while :; do
        sleep 60 || :
    done
}

locasos_guard_main() {
    [ "$#" -eq 0 ] || locasos_guard_refuse
    if locasos_guard_check /sys; then
        echo 'LOCASOS_INSTALLER_VM_CHECK_OK: review the virtual disk before confirming installation.'
    else
        locasos_guard_refuse
    fi
}

# Sourcing this file defines functions for fixture tests without running them.
# Executing it (including with `sh guard-vm.sh`) cannot select a fixture root.
case "$0" in
    guard-vm.sh|*/guard-vm.sh) locasos_guard_main "$@" ;;
esac
