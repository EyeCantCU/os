# Stereo Archive Analysis and Withdrawal Process

```dot
digraph archive_withdrawal {
    rankdir=TD;
    node [shape=box, style=filled, fontname="Arial", fontsize=10];
    edge [fontname="Arial", fontsize=9];
    compound=true;

    // Input Data cluster
    subgraph cluster_input {
        label="Input Data";
        style=filled;
        fillcolor="#f9f9f9";

        I1 [label="resolved/build/{arch}/\nBuild dependencies", shape=folder, fillcolor="#e3f2fd"];
        I2 [label="resolved/images/{arch}/\nImage dependencies", shape=folder, fillcolor="#f1f8e9"];
        I3 [label="resolved/vms/{arch}/\nVM dependencies", shape=folder, fillcolor="#fff8e1"];
        I4 [label="resolved/seeds/{arch}/\nSeed dependencies\n(optional)", shape=folder, fillcolor="#fce4ec"];
        I5 [label="resolved/version-streams/{arch}/\nVersion stream dependencies\n(optional)", shape=folder, fillcolor="#e8f5e8"];
    }

    // Archive Analysis cluster
    subgraph cluster_analysis {
        label="Archive Analysis";
        style=filled;
        fillcolor="#fce4ec";

        AA [label="stereo archive\n--duration 365\n--generate-withdrawn", fillcolor="#fce4ec"];
        AA1 [label="Archive Criteria\nCross-Architecture Analysis", shape=diamond, fillcolor="#f8bbd9"];

        AA2 [label="Age Analysis\nPackages > 365 days\n(consolidated across archs)", fillcolor="#fce4ec"];
        AA3 [label="Dependency Analysis\nPer Architecture", fillcolor="#fce4ec"];
        AA4 [label="Safety Check\nMust meet ALL criteria\nacross ALL architectures", fillcolor="#ffcdd2"];

        AA3A [label="No reverse dependencies\non ANY architecture", fillcolor="#fce4ec"];
        AA3B [label="Not in active builds\non ANY architecture", fillcolor="#fce4ec"];
        AA3C [label="Not most recent version\nif still built", fillcolor="#fce4ec"];
        AA3D [label="Not in images/VMs/seeds/version-streams\non ANY architecture", fillcolor="#fce4ec"];

        AA -> AA1;
        AA1 -> AA2;
        AA1 -> AA3;
        AA3 -> AA3A;
        AA3 -> AA3B;
        AA3 -> AA3C;
        AA3 -> AA3D;
        AA1 -> AA4;
    }

    // Archive Outputs cluster
    subgraph cluster_outputs {
        label="Archive Outputs";
        style=filled;
        fillcolor="#f0f0f0";

        AO1 [label="archive/\nArchive candidates\nper repository", shape=folder, fillcolor="#ffeb3b"];
        AO2 [label="retain/\nRetained packages\nper repository", shape=folder, fillcolor="#4caf50"];
        AO3 [label="withdrawn-packages.txt\nPer repository directory:\n• os/withdrawn-packages.txt\n• extra-packages/withdrawn-packages.txt\n• enterprise-packages/withdrawn-packages.txt", shape=note, fillcolor="#fff9c4"];
    }

    // Withdrawal Process cluster
    subgraph cluster_withdrawal {
        label="Withdrawal Process";
        style=filled;
        fillcolor="#f3e5f5";

        WP [label="stereo withdraw\n--output-dir withdrawn-indexes\n--signing-key melange.rsa", fillcolor="#f3e5f5"];
        WP1 [label="For each repository with\nwithdrawn-packages.txt", fillcolor="#f3e5f5"];
        WP2 [label="Download Current APKINDEX\nPer Architecture", fillcolor="#f3e5f5"];
        WP3 [label="Parse withdrawn-packages.txt\nExtract APK filenames\ne.g., firefox-127.0.2-r0.apk", fillcolor="#f3e5f5"];
        WP4 [label="Remove Packages\nPer Architecture", fillcolor="#f3e5f5"];
        WP5 [label="Sign Modified APKINDEX\nwith RSA key", fillcolor="#f3e5f5"];
        WP6 [label="Save to Output Directory\nwithdrawn-indexes/", fillcolor="#f3e5f5"];

        // Download sources
        WP2A [label="packages.wolfi.dev/os\n{arch}/APKINDEX.tar.gz", shape=ellipse, fillcolor="#e8f5e8"];
        WP2B [label="packages.cgr.dev/extras\n{arch}/APKINDEX.tar.gz", shape=ellipse, fillcolor="#e8f5e8"];
        WP2C [label="apk.cgr.dev/chainguard-private\n{arch}/APKINDEX.tar.gz", shape=ellipse, fillcolor="#e8f5e8"];

        WP -> WP1 -> WP2;
        WP1 -> WP3;
        WP2 -> WP2A;
        WP2 -> WP2B;
        WP2 -> WP2C;
        WP2A -> WP4;
        WP2B -> WP4;
        WP2C -> WP4;
        WP3 -> WP4;
        WP4 -> WP5 -> WP6;
    }

    // Output Structure cluster
    subgraph cluster_final {
        label="Output Structure";
        style=filled;
        fillcolor="#e1bee7";

        WOS [label="withdrawn-indexes/\nModified APKINDEX files", fillcolor="#e1bee7"];
        WOS1 [label="os/\n{arch}/APKINDEX.tar.gz", shape=folder, fillcolor="#f0f0f0"];
        WOS2 [label="extra-packages/\n{arch}/APKINDEX.tar.gz", shape=folder, fillcolor="#f0f0f0"];
        WOS3 [label="enterprise-packages/\n{arch}/APKINDEX.tar.gz", shape=folder, fillcolor="#f0f0f0"];

        WOS -> WOS1;
        WOS -> WOS2;
        WOS -> WOS3;
    }

    // Cross-cluster connections
    I1 -> AA [ltail=cluster_input, lhead=cluster_analysis];
    I2 -> AA [ltail=cluster_input, lhead=cluster_analysis];
    I3 -> AA [ltail=cluster_input, lhead=cluster_analysis];
    I4 -> AA [ltail=cluster_input, lhead=cluster_analysis];
    I5 -> AA [ltail=cluster_input, lhead=cluster_analysis];

    AA4 -> AO1 [ltail=cluster_analysis, lhead=cluster_outputs];
    AA4 -> AO2 [ltail=cluster_analysis, lhead=cluster_outputs];
    AA4 -> AO3 [ltail=cluster_analysis, lhead=cluster_outputs];

    AO3 -> WP1 [ltail=cluster_outputs, lhead=cluster_withdrawal];
    WP6 -> WOS [ltail=cluster_withdrawal, lhead=cluster_final];
}
```

## Archive Decision Matrix

| Criteria | Check Scope | Result |
|----------|-------------|---------|
| Age > 365 days | Consolidated across architectures | ✅ Archive candidate |
| No reverse deps | **ALL** architectures | ✅ Safe to remove |
| Not in active builds | **ALL** architectures | ✅ Won't break builds |
| Not most recent version | If still built | ✅ Keep latest only |
| Not in images/VMs/seeds/version-streams | **ALL** architectures | ✅ Won't break deployments |

**Key Safety Feature:** Packages only archived if criteria met across **ALL** supported architectures