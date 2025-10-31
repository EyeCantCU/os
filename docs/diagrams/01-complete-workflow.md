# Stereo Archive Process - Complete Workflow

```dot
digraph workflow {
    rankdir=TD;
    node [shape=box, style=filled, fontname="Arial", fontsize=10];
    edge [fontname="Arial", fontsize=9];

    // Start and End
    A [label="Start:\nPackage Archive Process", fillcolor="#e1f5fe"];
    K [label="End:\nPackages Archived", fillcolor="#c8e6c9"];

    // Decision points
    B [label="Dependencies\nPre-computed?", shape=diamond, fillcolor="#f5f5f5"];
    G [label="Validation\nResults", shape=diamond, fillcolor="#f5f5f5"];

    // Main phases
    C [label="Run Dependencies\nCommands", fillcolor="#fff3e0"];
    D [label="Run Archive\nAnalysis", fillcolor="#fce4ec"];
    E [label="Withdrawal\nProcess", fillcolor="#f3e5f5"];
    F [label="Validation\nTesting", fillcolor="#e0f2f1"];
    H [label="Deploy Modified\nIndexes", fillcolor="#c8e6c9"];
    I [label="Review Unresolved\nDependencies", fillcolor="#ffcdd2"];
    J [label="Adjust withdrawn-packages.txt\nor Fix Dependencies", fillcolor="#ffcdd2"];

    // Dependencies commands
    C1 [label="stereo build-dependencies", fillcolor="#fff3e0"];
    C2 [label="stereo image-dependencies\n< tfplan.json", fillcolor="#fff3e0"];
    C3 [label="stereo vm-dependencies", fillcolor="#fff3e0"];
    C4 [label="stereo seed-dependencies\n(optional)", fillcolor="#fff3e0"];

    // Dependencies outputs
    C1A [label="resolved/build/{arch}/\nBuild deps per repo", shape=folder, fillcolor="#f0f0f0"];
    C2A [label="resolved/images/{arch}/\nImage deps public/private", shape=folder, fillcolor="#f0f0f0"];
    C3A [label="resolved/vms/{arch}/\nVM dependencies", shape=folder, fillcolor="#f0f0f0"];
    C4A [label="resolved/seeds/{arch}/\nManual seed deps", shape=folder, fillcolor="#f0f0f0"];

    // Archive process
    D1 [label="Archive Analysis:\n• Age > 365 days\n• No reverse dependencies\n• Not in active builds\n• Not in images/VMs/seeds", fillcolor="#fce4ec"];
    D2 [label="archive/\nArchive candidates", shape=folder, fillcolor="#f0f0f0"];
    D3 [label="retain/\nRetained packages", shape=folder, fillcolor="#f0f0f0"];
    D4 [label="withdrawn-packages.txt\nPer repository", shape=note, fillcolor="#fff9c4"];

    // Withdraw process
    E1 [label="Process each repo:\n• Download APKINDEX.tar.gz\n• Remove withdrawn packages\n• Save modified index", fillcolor="#f3e5f5"];
    E2 [label="withdrawn-indexes/\nrepo/arch/APKINDEX.tar.gz", shape=folder, fillcolor="#f0f0f0"];

    // Validation commands
    F1 [label="stereo build-dependencies\n--use-withdrawn", fillcolor="#e0f2f1"];
    F2 [label="stereo image-dependencies\n--use-withdrawn < tfplan.json", fillcolor="#e0f2f1"];
    F3 [label="stereo vm-dependencies\n--use-withdrawn", fillcolor="#e0f2f1"];
    F4 [label="stereo seed-dependencies\n--use-withdrawn\n(optional)", fillcolor="#e0f2f1"];

    // Validation outputs
    F1A [label="withdrawn-test/resolved/build/\nwithdrawn-test/unresolved/build/", shape=folder, fillcolor="#f0f0f0"];
    F2A [label="withdrawn-test/resolved/images/\nwithdrawn-test/unresolved/images/", shape=folder, fillcolor="#f0f0f0"];
    F3A [label="withdrawn-test/resolved/vms/\nwithdrawn-test/unresolved/vms/", shape=folder, fillcolor="#f0f0f0"];
    F4A [label="withdrawn-test/resolved/seeds/\nwithdrawn-test/unresolved/seeds/", shape=folder, fillcolor="#f0f0f0"];

    // Main flow
    A -> B;
    B -> C [label="No"];
    B -> D [label="Yes"];

    // Dependencies phase
    C -> C1;
    C -> C2;
    C -> C3;
    C -> C4;

    C1 -> C1A;
    C2 -> C2A;
    C3 -> C3A;
    C4 -> C4A;

    C1A -> D;
    C2A -> D;
    C3A -> D;
    C4A -> D;

    // Archive phase
    D -> D1 [label="stereo archive\n--generate-withdrawn\n--duration 365"];
    D1 -> D2;
    D1 -> D3;
    D1 -> D4;

    // Withdraw phase
    D4 -> E [label="stereo withdraw\n--output-dir withdrawn-indexes"];
    E -> E1;
    E1 -> E2;

    // Validation phase
    E2 -> F;
    F -> F1;
    F -> F2;
    F -> F3;
    F -> F4;

    F1 -> F1A;
    F2 -> F2A;
    F3 -> F3A;
    F4 -> F4A;

    F1A -> G;
    F2A -> G;
    F3A -> G;
    F4A -> G;

    // Results
    G -> H [label="Success"];
    G -> I [label="Failures"];

    I -> J;
    J -> E;

    H -> K;
}
```

## Key Process Phases

1. **Dependencies Resolution** (Orange): Pre-compute all package dependencies
2. **Archive Analysis** (Pink): Identify packages for archival based on criteria
3. **Withdrawal** (Purple): Create modified APKINDEX files with packages removed
4. **Validation** (Green): Test dependency resolution with withdrawn packages