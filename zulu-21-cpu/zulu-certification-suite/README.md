# Zulu Certification Suite

# Overview

This directory contains **jtreg** test suites designed to validate key aspects of the Java runtime and Zulu builds.

---

## Test Directory Structure

- **TEST.ROOT**  
  Required in the root of every test directory to identify it as a valid jtreg test root.

---

## Test Suites

### ./crash_string

- Tests verifying crash string outputs to ensure proper formatting and content.

### ./version

- Tests focused on validating Java version information, including:
    - Correct output of the `java -version` command
    - Validation of Zulu and OpenJDK version strings in the output
    - Checking properties related to version

### ./zulu_files

- Tests that verify the presence and correctness of important files in the Zulu build, such as:
    - License files and related legal documentation
    - Readme and Welcome.html files, ensuring content is up-to-date and properly formatted

---

## Run

Please, follow the official jtreg instructions and build jtreg harness https://openjdk.org/jtreg/build.html.

```
export TESTED_ZULU=PathToTestedZulu
export JTREG_PATH=PathToJTreg

$TESTED_ZULU/bin/java -jar $JTREG_PATH/lib/jtreg.jar -verbose  -retain:all  \
  -e:MAJOR_VERSION=${{vars.major-version}},JAVA_VERSION=${{package.version}},ZULU_VERSION=${{vars.zulu-version}},BRANDING=cg  \
  -jdk:$TESTED_ZULU .
```