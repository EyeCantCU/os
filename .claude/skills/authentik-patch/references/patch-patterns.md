# Authentik Enterprise Patch Patterns

This document describes common patterns for patching out authentik.enterprise module references in the OSS build.

## Overview

Authentik's upstream code unconditionally loads enterprise components despite licensing restrictions. The OSS build removes the `/authentik/enterprise` directory and patches all references to prevent import errors and runtime failures.

## Pattern Categories

### 1. Import Statement Patterns

#### Pattern 1.1: Comment Out Module Imports

**When to use**: Direct imports of enterprise modules

**File types**: Python files (`.py`)

**Before**:
```python
from authentik.enterprise.license import LicenseKey
from authentik.core.models import User
```

**After**:
```python
#from authentik.enterprise.license import LicenseKey
from authentik.core.models import User
```

**Example files**:
- `authentik/admin/api/system.py`
- `authentik/blueprints/v1/importer.py`

---

#### Pattern 1.2: Comment Out Multi-line Enterprise Imports

**When to use**: Multiple enterprise imports in a block

**File types**: Python files (`.py`)

**Before**:
```python
from authentik.enterprise.providers.google_workspace.models import (
    GoogleWorkspaceProviderGroup,
    GoogleWorkspaceProviderUser,
)
from authentik.enterprise.providers.microsoft_entra.models import (
    MicrosoftEntraProviderGroup,
    MicrosoftEntraProviderUser,
)
```

**After**:
```python
#from authentik.enterprise.providers.google_workspace.models import (
#    GoogleWorkspaceProviderGroup,
#    GoogleWorkspaceProviderUser,
#)
#from authentik.enterprise.providers.microsoft_entra.models import (
#    MicrosoftEntraProviderGroup,
#    MicrosoftEntraProviderUser,
#)
```

**Example files**:
- `authentik/blueprints/v1/importer.py`

---

### 2. License Check Patterns

#### Pattern 2.1: Replace License Validation with None

**When to use**: Code that checks license validity

**File types**: Python API files

**Before**:
```python
"openssl_fips_enabled": (
    backend._fips_enabled if LicenseKey.get_total().status().is_valid else None
),
```

**After**:
```python
"openssl_fips_enabled": (
    #backend._fips_enabled if LicenseKey.get_total().status().is_valid else None
    None
),
```

**Example files**:
- `authentik/admin/api/system.py`

**Note**: Always default to `None` or `False` for license-dependent features

---

### 3. List/Collection Patterns

#### Pattern 3.1: Comment Out Model Exclusions

**When to use**: Enterprise models in exclusion lists

**File types**: Python files with model definitions

**Before**:
```python
excluded = [
    FlowToken,
    LicenseUsage,
    SCIMProviderGroup,
    Tenant,
]
```

**After**:
```python
excluded = [
    FlowToken,
    #LicenseUsage,
    SCIMProviderGroup,
    Tenant,
]
```

**Example files**:
- `authentik/blueprints/v1/importer.py`

---

### 4. Class Inheritance Patterns

#### Pattern 4.1: Remove ConditionalInheritance Calls

**When to use**: Classes that conditionally inherit enterprise mixins

**File types**: Python API/serializer files

**Before**:
```python
class UserViewSet(
    ConditionalInheritance("authentik.enterprise.reports.api.reports.ExportMixin"),
    UsedByMixin,
    ModelViewSet,
):
    pass
```

**After**:
```python
class UserViewSet(
    #ConditionalInheritance("authentik.enterprise.reports.api.reports.ExportMixin"),
    UsedByMixin,
    ModelViewSet,
):
    pass
```

**Example files**:
- `authentik/core/api/users.py`
- `authentik/events/api/events.py`
- `authentik/endpoints/connectors/agent/api/connectors.py`

**Note**: `ConditionalInheritance` is a Django utility that loads mixins if the module path exists. Commenting these prevents MRO (Method Resolution Order) errors.

---

#### Pattern 4.2: Create Empty Stub Classes

**When to use**: When code expects a mixin class to exist

**File types**: Python files

**Before**:
```python
from authentik.enterprise.api import EnterpriseRequiredMixin

class MyView(EnterpriseRequiredMixin, BaseView):
    pass
```

**After**:
```python
#from authentik.enterprise.api import EnterpriseRequiredMixin

# Stub for removed enterprise mixin
class EnterpriseRequiredMixin:
    pass

class MyView(EnterpriseRequiredMixin, BaseView):
    pass
```

**Example files**:
- `authentik/endpoints/api/stages.py`

---

### 5. Test Pattern

#### Pattern 5.1: Comment Out Enterprise Test Assertions

**When to use**: Tests that verify enterprise app configuration

**File types**: Test files (`test_*.py`)

**Before**:
```python
def test_apps_use_managed_app_config(self):
    for app in get_apps():
        if app.name.startswith("authentik.enterprise"):
            self.assertIn(EnterpriseConfig, app.__class__.__bases__)
        else:
            self.assertIn(ManagedAppConfig, app.__class__.__bases__)
```

**After**:
```python
def test_apps_use_managed_app_config(self):
    for app in get_apps():
        #if app.name.startswith("authentik.enterprise"):
        #    self.assertIn(EnterpriseConfig, app.__class__.__bases__)
        #else:
        self.assertIn(ManagedAppConfig, app.__class__.__bases__)
```

**Example files**:
- `authentik/blueprints/tests/test_managed_app_config.py`

---

### 6. Configuration Patterns

#### Pattern 6.1: Remove Enterprise from Django App List

**When to use**: Django settings that load enterprise app

**File types**: `settings.py`

**Before**:
```python
TENANT_APPS = [
    "authentik.core",
    "authentik.enterprise",
    "authentik.flows",
]
```

**After**:
```python
TENANT_APPS = [
    "authentik.core",
    #"authentik.enterprise",
    "authentik.flows",
]
```

**Example files**:
- `authentik/root/settings.py`

**Note**: This is typically the first patch that must be applied

---

### 7. Frontend Patterns

#### Pattern 7.1: Comment Out Enterprise Provider API Calls

**When to use**: TypeScript/JavaScript that fetches enterprise provider data

**File types**: `.ts`, `.js`

**Before**:
```typescript
const statuses = [
    await this.fetchStatus(api.providersScimList()),
    await this.fetchStatus(api.providersGoogleWorkspaceList()),
    await this.fetchStatus(api.providersMicrosoftEntraList()),
    await this.fetchStatus(api.providersLdapList()),
];
```

**After**:
```typescript
const statuses = [
    await this.fetchStatus(api.providersScimList()),
    /* await this.fetchStatus(api.providersGoogleWorkspaceList()), */
    /* await this.fetchStatus(api.providersMicrosoftEntraList()), */
    await this.fetchStatus(api.providersLdapList()),
];
```

**Example files**:
- `web/src/admin/admin-overview/charts/SyncStatusChart.ts`

**Note**: Use `/* */` style comments for JavaScript/TypeScript

---

### 8. Defensive Coding Patterns

#### Pattern 8.1: Add None-Safe Unpacking

**When to use**: Code that unpacks kwargs that may be None when enterprise module is removed

**File types**: Middleware, signal handlers

**Before**:
```python
def m2m_changed_handler(sender, instance, **thread_kwargs):
    Model.objects.create(
        **thread_kwargs,
    )
```

**After**:
```python
def m2m_changed_handler(sender, instance, **thread_kwargs):
    Model.objects.create(
        **(thread_kwargs or {}),
    )
```

**Example files**:
- `authentik/events/middleware.py`

**Reason**: Prevents `TypeError: argument after ** must be a mapping, not NoneType`

---

### 9. Serializer Patterns

#### Pattern 9.1: Comment Out Serializer Mixin Inheritance

**When to use**: Provider serializers that inherit enterprise mixins

**File types**: API serializer files

**Before**:
```python
class RadiusProviderSerializer(
    ConditionalInheritance("authentik.enterprise.providers.radius.api.RadiusProviderSerializerMixin"),
    ProviderSerializer,
):
    pass
```

**After**:
```python
class RadiusProviderSerializer(
    #ConditionalInheritance("authentik.enterprise.providers.radius.api.RadiusProviderSerializerMixin"),
    ProviderSerializer,
):
    pass
```

**Example files**:
- `authentik/providers/radius/api/providers.py`
- `authentik/providers/scim/api/providers.py`

---

### 10. Search Field Patterns

#### Pattern 10.1: Comment Out Enterprise Search Field Imports

**When to use**: Advanced search fields from enterprise module

**File types**: API files with search functionality

**Before**:
```python
from authentik.enterprise.search.fields import (
    JSONSearchField,
    ChoiceSearchField,
)
```

**After**:
```python
#from authentik.enterprise.search.fields import (
#    JSONSearchField,
#    ChoiceSearchField,
#)
```

**Example files**:
- Various API files with advanced search

---

### 11. Model Import Patterns

#### Pattern 11.1: Comment Out Enterprise Device/Endpoint Imports

**When to use**: Endpoint device management imports

**File types**: Model files, API files

**Before**:
```python
from authentik.enterprise.stages.authenticator_endpoint_gdtc.models import (
    EndpointDevice,
    EndpointDeviceConnection,
)
```

**After**:
```python
#from authentik.enterprise.stages.authenticator_endpoint_gdtc.models import (
#    EndpointDevice,
#    EndpointDeviceConnection,
#)
```

---

### 12. Provider Patterns

#### Pattern 12.1: Comment Out SSF Stream Event Imports

**When to use**: Shared Signals Framework imports

**File types**: Provider model files

**Before**:
```python
from authentik.enterprise.providers.ssf.models import StreamEvent
```

**After**:
```python
#from authentik.enterprise.providers.ssf.models import StreamEvent
```

---

### 13. OAuth Patterns

#### Pattern 13.1: Comment Out Enterprise OAuth Backend Imports

**When to use**: SCIM provider OAuth backends

**File types**: SCIM provider files

**Before**:
```python
from authentik.enterprise.providers.scim.auth_oauth2 import SCIMOAuth2Backend
```

**After**:
```python
#from authentik.enterprise.providers.scim.auth_oauth2 import SCIMOAuth2Backend
```

**Example files**:
- `authentik/providers/scim/models.py`

---

### 14. App Config Patterns

#### Pattern 14.1: Comment Out Enterprise App Config Imports

**When to use**: Enterprise app configuration checks

**File types**: Test files, app config files

**Before**:
```python
from authentik.enterprise.apps import EnterpriseConfig
```

**After**:
```python
#from authentik.enterprise.apps import EnterpriseConfig
```

**Example files**:
- `authentik/blueprints/tests/test_managed_app_config.py`

---

## Patch File Structure

All patches should follow this structure:

```diff
Description: Brief description of what's being patched
.
Longer explanation of why this patch is needed.
Author: Your Name <your.email@example.com>
Forwarded: not-needed

diff --git a/path/to/file.py b/path/to/file.py
index oldsha..newsha mode
--- a/path/to/file.py
+++ b/path/to/file.py
@@ -line,count +line,count @@ context
 unchanged line
-removed line
+added line
 unchanged line
```

## Patch Application Order

The patches should be applied in this order in the melange YAML:

1. `root.settings.patch` - Remove enterprise from Django apps
2. `enterprise.patch` - Comment out all enterprise imports and references
3. `enterprise.mro.patch` - Fix MRO issues from removed inheritance
4. `frontend-sync-chart.patch` - Remove enterprise UI elements
5. `middleware-m2m-fix.patch` - Add defensive coding for runtime edge cases

## Tips for Creating New Patches

1. **Always test patches apply cleanly**: Use `git apply --check patch_file.patch`
2. **Keep patches focused**: One patch per logical change category
3. **Include context**: Provide 3 lines of context before and after changes
4. **Document thoroughly**: Add clear descriptions explaining why the patch is needed
5. **Test the build**: Ensure authentik starts and core functionality works after patching

## Common Gotchas

- **Don't forget frontend patches**: TypeScript files also reference enterprise APIs
- **Check for ConditionalInheritance**: These can cause subtle MRO errors if not commented out
- **Test signal handlers**: Middleware that receives enterprise-initialized kwargs needs defensive coding
- **Verify test files**: Tests may assert enterprise functionality exists
- **Update all variants**: Both authentik.yaml and authentik-fips.yaml need the same patches

## Reference Files

Key patch files:
- `enterprise-packages/authentik/root.settings.patch` - Django app configuration
- `enterprise-packages/authentik/enterprise.patch` - Main import/reference cleanup (largest file)
- `enterprise-packages/authentik/enterprise.mro.patch` - Inheritance fixes
- `enterprise-packages/authentik/frontend-sync-chart.patch` - UI updates
- `enterprise-packages/authentik/middleware-m2m-fix.patch` - Runtime safety
