Name: ggupgrade
Version: %{ggupgrade_version}
# Release is a way of versioning the spec file.
# Only bump the Release if shipping ggupgrade without also bumping the
# ggugprade_version (ie: VERSION).
Release: %{ggupgrade_rpm_release}%{?dist}
Summary: %{summary}
License: %{license}
URL: https://github.com/GreengageDB/ggupgrade
Source0: %{name}.tar.gz
Prefix: /usr/local/bin
Requires: openssh rsync >= 3.0

%description
The ggupgrade package contains ggupgrade which performs in-place upgrades
without the need for additional hardware, disk space, and with less downtime.

%prep
# If the ggupgrade_version macro is not defined, it gets interpreted as a
# literal string, use %% to escape it
if [ %{ggupgrade_version} = '%%{ggupgrade_version}' ] ; then
    echo "The macro (variable) ggupgrade_version must be supplied as rpmbuild ... --define='ggupgrade_version [VERSION]'"
    exit 1
fi

%setup -q -c -n %{name}

%install
# executables
mkdir -p %{buildroot}%{prefix}
mv ggupgrade %{buildroot}%{prefix}

# additional files
mkdir -p %{buildroot}%{prefix}/greengage/%{name}
mv data-migration-scripts %{buildroot}%{prefix}/greengage/%{name}
mv ggupgrade_config %{buildroot}%{prefix}/greengage/%{name}
mv ggupgrade.bash %{buildroot}%{prefix}/greengage/%{name}
mv open_source_licenses.txt %{buildroot}%{prefix}/greengage/%{name}


%files
# executables
%{prefix}/ggupgrade

# additional files
%dir %{prefix}/greengage
%dir %{prefix}/greengage/%{name}
%{prefix}/greengage/%{name}/data-migration-scripts
%config %{prefix}/greengage/%{name}/ggupgrade_config
%{prefix}/greengage/%{name}/ggupgrade.bash
%{prefix}/greengage/%{name}/open_source_licenses.txt
