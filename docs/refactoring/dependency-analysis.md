# ListingKit Module Dependency Analysis

**Analysis Date:** 2026-06-08

## File Distribution Statistics

| Module | Files (non-test) | Description |
|--------|------------------|-------------|
| Root (根目录) | 303 | Main refactoring target |
| api/ | ~37 | HTTP API handlers |
| generation/ | ~24 | Generation queue core types |
| submission/ | ~8 | Submission readiness (needs expansion) |
| store/ | ~9 | Data persistence |
| workflow/ | ~5 | Temporal workflow definitions |
| service/ | 0 | To be created |
| core/ | 0 | To be created |
| httpapi/ | ~37 | HTTP API layer |
| reviewstore/ | ~5 | Review session storage |
| studiostore/ | ~4 | Studio storage |
| temporal/ | ~14 | Temporal client wrappers |
| sheinsync/ | ~17 | SHEIN sync logic |
| workspace/ | ~2 | Workspace bridges |
| sds/ | ~2 | SDS integration |
| studio/ | ~4 | Studio session management |
| preview/ | ~1 | Preview builders |
| revision/ | ~2 | Revision history |
| admin/ | ~2 | Admin services |
| tenantctx/ | ~1 | Tenant context |
| data/ | 0 | Data directory (empty) |
| models/ | 0 | Models directory (empty) |
| tmp/ | ~1 | Temporary files |
| upload/ | 0 | Upload directory (empty) |

**Total sub-package files:** ~207  
**Root directory files:** 303  
**Grand total:** 510 files (including tests would be higher)

## Root Directory File Classification

### Should move to core/ (~20 files)
- interfaces.go - Core interfaces
- processor.go - Processor interface
- model*.go - Core models (14 files)
- assembler.go - Assembler utility
- *_helpers.go - Helper utilities (4 files)

### Should move to service/ (~50 files)
- service.go - Main service entry
- service_*.go - Service implementations (~45 files)
- task_studio*.go - Studio task services (~5 files)

### Should move to submission/ (~30 files)
- submit_*.go - Submission related (~15 files)
- shein_submit*.go - SHEIN submission (~15 files)

### Should remain in root (~203 files)
- generation_*.go - Generation domain implementation
- task_generation_*.go - Generation tasks
- workflow_*.go - Workflow implementations
- phase*_test.go - Boundary tests
- Other domain-specific files

## Migration Priority

**High Priority (Week 1-2):**
1. Extract core/ package (~20 files)
2. Organize service/ package (~50 files)

**Medium Priority (Week 3):**
3. Expand submission/ package (~30 files)

**Low Priority (Week 4-5):**
4. Documentation completion
5. Clean up boundary tests (optional)

## Dependency Flow Diagram

```
app/ (Application Layer - Assembly)
  ↓
listingkit/ (Business Layer)
  ├── core/ (Core Interfaces & Models) ← Foundation
  ├── service/ (Service Implementations)
  │   ├── generation
  │   ├── submission
  │   ├── revision
  │   └── studio
  ├── submission/ (Submission Readiness)
  ├── generation/ (Generation Queue)
  ├── workflow/ (Temporal Workflows)
  ├── store/ (Data Persistence)
  └── api/ (HTTP API)
```

## Key Observations

1. **Root directory is overloaded**: 303 files is too many for a single package
2. **Clear domain boundaries exist**: generation, submission, workflow are well-defined
3. **Missing abstraction layers**: No dedicated core/ or service/ packages yet
4. **Test distribution**: Many test files are in root alongside implementation
5. **Sub-module maturity**: Some sub-modules (store, temporal) are well-organized

## Recommended Actions

1. **Immediate**: Create core/ and service/ packages
2. **Short-term**: Move clearly-scoped files to appropriate sub-packages
3. **Long-term**: Consider further splitting large sub-modules (api/, generation/)

## Notes

- Actual file counts may vary slightly due to recent changes
- Test files (_test.go) are excluded from non-test counts
- Empty directories (data/, models/, upload/) should be cleaned up or utilized
