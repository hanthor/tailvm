# Issue: Implement Quickget-Style Dynamic Image Resolver & Testing Plan

## 📌 Goal
Implement a dynamic OS image catalog and downloader inside `tailvm` modeled after Quickemu's **`quickget`** utility. This feature allows users to dynamically list, search, and resolve the latest official ISO/QCOW2 URLs for Ubuntu, Fedora, Debian, Arch, Windows, and macOS, seamlessly integrating them into KubeVirt CDI DataVolumes during provisioning. Establish a testing plan for this resolver.

## 🛠️ Design & Requirements

### 1. Dynamic Catalog Engine (`quickget.go`)
- **Static Catalogs**: Maintain a structured local mapping (JSON/Go structs) of major Linux distributions and operating systems.
- **Dynamic Scrapers / API**: Implement light scrapers or query online catalogs (e.g. Ubuntu cloud image indexes, Debian release mirrors, Quickemu's own metadata registries) to dynamically resolve the *latest* stable URL for the requested OS (amd64/arm64).
- **Subcommands**:
  - `tailvm catalog`: Displays a clean tabular list of all supported operating systems, architectures, and mirror sources.
  - `tailvm create [name] --os ubuntu-noble`: Automatically triggers the resolver, finds the Noble Numbat cloud image URL, and applies it to the CDI DataVolume creation flow.

### 2. TUI Integration
- Update the Bubble Tea TUI VM Creation Form:
  - When configuring the disk source, the user can choose "Browse Catalog".
  - This opens a fuzzy-searchable list of OS templates. Clicking a template automatically resolves the latest URL, populates the fields, and configures the optimized resources (CPU, RAM, VM Specs) for that specific OS!

---

## 🧪 Testing Plan

### 1. Unit Tests (`quickget_test.go`)
- **Catalog Integrity**: Verify that the built-in OS catalog matches schema rules (contains valid IDs, default resources, and mirror templates).
- **Resolution Mocking**: Mock the HTTP client inside `quickget` and test that our parsing logic correctly extracts URLs from mock HTML/JSON index pages (e.g., parsing Ubuntu's `SHA256SUMS` or release directories).
- **Arch Matching**: Verify that request filters return `amd64` or `arm64` images matching the cluster node's physical architecture.

### 2. Integration Tests (`integration_quickget_test.go`)
- **Live Upstream Validation**: Run automated checks that query a representative sample of upstream mirrors (e.g. `releases.ubuntu.com`, `dl.fedoraproject.org`) to assert that the resolved URLs are active, return `200 OK`, and have valid Content-Lengths.
- **TUI Fuzzy Search Assertions**: Mock keyboard events inside the TUI "OS Selector" list and assert that selection updates the active VM model correctly.

---

## 📋 Sub-Issues & Granular Tasks Checklist

- [ ] **Sub-Issue 7.1: Image Catalog Mapping & Structures**
  - [ ] Implement `quickget.go` static OS dictionaries.
  - [ ] Build multi-arch selection filters (amd64/arm64).
- [ ] **Sub-Issue 7.2: Scrapers & Index parsers**
  - [ ] Add HTML parser handlers to dynamically scrape release indices for Debian and Fedora.
  - [ ] Implement local HTTP caches for resolved URLs.
- [ ] **Sub-Issue 7.3: TUI Integration**
  - [ ] Build dynamic "Catalog Browser" panel using the bubbles list widget.
  - [ ] Map click/select events to automatically fill VM form structures.
- [ ] **Sub-Issue 7.4: Unit & Integration Tests Implementation**
  - [ ] Implement `quickget_test.go` checking parsing schemas and static maps.
  - [ ] Implement `integration_quickget_test.go` performing live upstream validation checks.

