# Tier 4: Driver Instructions — Ship It on a VM

Step-by-step instructions for all manual tasks. Run commands from your local machine unless noted otherwise.

## Prerequisites

Make sure you have:
- `replicated` CLI installed and authenticated
- SSH key pair for CMX VMs
- A test customer in Vendor Portal with **Embedded Cluster Enabled** and **Airgap Download Enabled**

```bash
# Verify CLI works
export REPLICATED_APP=astrid
replicated app ls
```

---

## Phase 0: Enable Embedded Cluster on Your Channel

**This is required before anything else.** By default, Helm-chart-only apps have `isHelmOnly: true` on their channels, which blocks EC installs.

### Step 0.1: Get your app ID and channel ID

```bash
replicated app ls --output json
replicated channel ls --output json
```

Note the `id` fields for your app and the `Unstable` channel.

### Step 0.2: Disable Helm-only mode

```bash
replicated api put \
  "/v3/app/<APP_ID>/channel/<CHANNEL_ID>" \
  --body '{"name":"Unstable","isHelmOnly":false}'
```

Replace `<APP_ID>` and `<CHANNEL_ID>` with the values from step 0.1.

### Step 0.3: Enable automatic air-gap builds (needed for rubric 4.3 later)

In Vendor Portal: go to **Channels** > **Unstable** > gear icon > enable **"Automatic Air Gap Builds"**.

Or via API:
```bash
replicated api put \
  "/v3/app/<APP_ID>/channel/<CHANNEL_ID>" \
  --body '{"name":"Unstable","isHelmOnly":false,"buildAirgapAutomatically":true}'
```

---

## Phase 1: Create Release & Test EC Install (Rubric 4.1)

### Step 1.1: Push your branch and create the release

From your local machine, in the `feat/tier-4` worktree:

```bash
# Push the branch
git push -u origin feat/tier-4

# Create and promote a release
replicated release create --promote Unstable --version "ec-test-1"
```

The `.replicated` file tells the CLI where to find charts and manifests. No `--yaml-dir` needed.

### Step 1.2: Verify the release

```bash
replicated release ls
```

You should see your release with `Unstable` in the `ACTIVE_CHANNELS` column. Also check in the Vendor Portal under **Releases** — the release should show an Embedded Cluster indicator.

### Step 1.3: Provision a CMX VM

```bash
replicated vm create \
  --distribution ubuntu \
  --version 22.04 \
  --instance-type r1.medium \
  --disk 100 \
  --ttl 4h \
  --name astrid-ec-test \
  --wait 5m
```

Wait for it to be ready:
```bash
replicated vm ls
```

Note the VM ID and IP address.

### Step 1.4: SSH into the VM

```bash
ssh <VM_IP>
```

The SSH user depends on the distribution (usually `ubuntu` for Ubuntu VMs, but CMX may use a different default — check the `replicated vm ls` output for connection info).

### Step 1.5: Get install commands from Vendor Portal

1. Go to https://vendor.replicated.com
2. Navigate to **Customers** > select your test customer
3. Click **Install instructions**
4. Choose **Embedded Cluster**
5. Select version **ec-test-1**
6. Copy the download command (a `curl` command)

### Step 1.6: Download and extract on the VM

Paste the curl command from the portal, then extract:

```bash
# Paste the curl command from the portal, something like:
curl -f https://replicated.app/embedded/astrid/unstable -H "Authorization: <license-id>" -o astrid-unstable.tgz

# Extract
tar xzf astrid-unstable.tgz
```

### Step 1.7: Run the installer

```bash
sudo ./astrid install
```

The installer will prompt you through these steps:
1. **Accept self-signed certificate** — type `y` and press Enter
2. **Set admin console password** — type a password you'll remember, confirm it
3. **Host preflight checks run automatically** — verifies disk, memory, CPU, latency
4. **Installs k0s, OpenEBS, embedded registry, admin console** — wait for this to complete (several minutes)

When complete, you'll see:
```
Visit the Admin Console to configure your application: http://<VM-IP>:30000
```

### Step 1.8: Configure in the Admin Console

1. Open `http://<VM-IP>:30000` in your browser
2. Log in with the password you set
3. **Upload license** — download it from Vendor Portal (Customers > your customer > Download license), then upload the `.yaml` file
4. **Config screen** — you'll see the config items we created (Database Type, Redis Type, Features). For a basic test:
   - Leave Database Type as **Embedded PostgreSQL**
   - Leave Redis Type as **Embedded Redis**
   - Click **Continue**
5. **Preflight checks** — review results, proceed if passing
6. **Deploy** — the admin console deploys your app

### Step 1.9: Verify (rubric acceptance criteria)

On the VM:
```bash
sudo k0s kubectl get pods -A
```

All pods should show `Running` or `Completed`. Look for:
- `astrid` deployment pod
- `astrid-postgresql` statefulset pod
- `astrid-redis-master` pod
- `astrid-replicated` (SDK) pod
- `kotsadm` pods
- `openebs` pods
- kube-system pods

Then open the app in a browser. The admin console shows the app URL, or you can check:
```bash
sudo k0s kubectl get svc -n default
```

**Screenshot this for your demo.**

---

## Phase 2: In-Place Upgrade Without Data Loss (Rubric 4.2)

### Step 2.1: Create test data in the app

Open the app in your browser and create identifiable data — e.g., create a user account, add some fitness entries. **Remember what you created** so you can verify it after upgrade.

### Step 2.2: Make a small change and create release 2

Back on your local machine, make a visible change so you can confirm the upgrade worked. For example, bump the chart appVersion:

Edit `chart/astrid/Chart.yaml` and change `appVersion` from `"0.1.0"` to `"0.2.0"`.

```bash
git add chart/astrid/Chart.yaml
git commit -m "chore: bump appVersion to 0.2.0 for upgrade test"
```

Create the second release:
```bash
replicated release create --promote Unstable --version "ec-test-2"
```

### Step 2.3: Trigger the upgrade

**Option A — From Admin Console (easiest):**

1. Open the admin console at `http://<VM-IP>:30000`
2. Go to the **Version History** tab
3. Click **Check for updates**
4. You should see `ec-test-2` appear
5. Click **Deploy** next to it

**Option B — From the VM CLI:**

1. Go back to Vendor Portal > Customers > Install Instructions > Embedded Cluster
2. Select version `ec-test-2`
3. Copy the new download command
4. On the VM:
```bash
curl -f <new-download-url> -o astrid-unstable-v2.tgz
tar xzf astrid-unstable-v2.tgz
sudo ./astrid upgrade --license license.yaml
```
5. Enter the installer password when prompted
6. Walk through the Configure > Upgrade > Finish screens

### Step 2.4: Verify data persists (rubric acceptance criteria)

1. Open the app in your browser
2. **Confirm your test data is still there** — the user account, fitness entries, etc.
3. Verify all pods are running:
```bash
sudo k0s kubectl get pods -A
```
4. Check PVCs are still bound:
```bash
sudo k0s kubectl get pvc -A
```

**Screenshot/record this for your demo.**

---

## Phase 3: Air-Gapped Install (Rubric 4.3)

### Step 3.1: Verify air-gap bundle was built

If you enabled automatic air-gap builds in Phase 0, the bundle should be ready. Check in the Vendor Portal:

1. Go to **Channels** > **Unstable**
2. In the release history, look for the air-gap bundle build status
3. Wait for it to complete if still building (can take several minutes)

### Step 3.2: Provision a CMX VM (with internet initially)

```bash
replicated vm create \
  --distribution ubuntu \
  --version 22.04 \
  --instance-type r1.medium \
  --disk 100 \
  --ttl 4h \
  --name astrid-airgap-test \
  --wait 5m
```

### Step 3.3: Get the network ID

```bash
replicated network ls
```

Note the `NETWORK_ID` associated with your new VM.

### Step 3.4: Download the air-gap bundle to the VM

While the VM still has internet access:

1. Go to Vendor Portal > **Customers** > select your test customer
2. Click **Install instructions**
3. Choose **Embedded Cluster**
4. Select **"Install in an air gap environment"**
5. Select the version
6. Copy the download command

SSH into the VM and run the download command:
```bash
ssh <VM_IP>

# Paste the curl command from the portal
curl -f <air-gap-download-url> -o astrid-airgap.tgz
```

### Step 3.5: Switch the VM to air-gap mode

Back on your local machine:
```bash
replicated network update <NETWORK_ID> --policy airgap
```

This cuts off all outbound internet access. SSH still works through CMX.

### Step 3.6: Extract and install on the air-gapped VM

SSH back into the VM:
```bash
ssh <VM_IP>

# Extract
tar xzf astrid-airgap.tgz

# Run the air-gap installer (note the --airgap flag pointing to the .airgap file)
sudo ./astrid install --license license.yaml --airgap astrid.airgap
```

The `.airgap` file name may differ — look at what was extracted. It contains all container images bundled in.

The install process is the same as the online install (password prompt, preflight checks, admin console), except all images are loaded from the local bundle instead of pulled from the internet.

### Step 3.7: Verify (rubric acceptance criteria)

```bash
sudo k0s kubectl get pods -A
```

All pods should be `Running`. Open the app in a browser.

**Screenshot/record this for your demo.**

---

## Phase 4: Verify App Icon and Name (Rubric 4.6)

During any of the above installs, when you first access the admin console and see the license upload page, the Astrid icon (neon pink star on space indigo background) and the title "Astrid" should be displayed.

**Screenshot the installer/admin console showing the icon and app name.**

---

## Phase 5: License Entitlement Gates Config (Rubric 4.7)

### Step 5.1: Create the license field in Vendor Portal

1. Go to Vendor Portal > **License Fields**
2. Click **Create License Field**
3. Add:
   - **Field name**: `google_oauth_enabled`
   - **Title**: Google OAuth Enabled
   - **Type**: Boolean
   - **Default**: `false`

### Step 5.2: Test with entitlement disabled

1. Ensure your test customer's license has `google_oauth_enabled = false`
2. Install with EC (or use an existing install)
3. On the config screen, the "Google OAuth Single Sign-On" toggle should **not appear** (it's hidden by `when: '{{repl LicenseFieldValue "google_oauth_enabled"}}'`)

### Step 5.3: Enable the entitlement

1. In Vendor Portal > **Customers** > your customer
2. Edit the license, set `google_oauth_enabled = true`
3. The customer can sync their license from the admin console (or re-upload)
4. Go back to the config screen — the Google OAuth toggle should **now appear**
5. Enable it, and the Client ID/Secret/Redirect URL fields should become visible

**Screenshot both states for your demo.**

---

## Cleanup

When done testing:
```bash
# List VMs
replicated vm ls

# Delete VMs
replicated vm rm <VM_ID>

# Or delete all at once if only test VMs exist
```

---

## Quick Reference: What to Demo for Each Rubric Item

| Rubric | What to Show |
|--------|-------------|
| 4.1 | Fresh VM → EC install → `sudo k0s kubectl get pods -A` all Running → app in browser |
| 4.2 | Create data → upgrade to v2 via admin console → data still there, pods Running |
| 4.3 | Air-gap bundle transferred → offline install → pods Running → app in browser |
| 4.6 | Screenshot of installer showing Astrid icon and name |
| 4.7 | Config screen without OAuth (entitlement off) → enable entitlement → OAuth fields appear |
| 5.0 | Config screen: embedded DB → pod Running. Switch to external → no DB pod, app uses external |
| 5.1 | Enable Google OAuth via config → feature works. Disable → feature gone |
| 5.2 | Install → upgrade → app still connects to DB (generated password survived) |
| 5.3 | Enter "abc" in port field → validation error. Enter "5432" → accepted |
| 5.4 | Show config screen with help text on every item |
