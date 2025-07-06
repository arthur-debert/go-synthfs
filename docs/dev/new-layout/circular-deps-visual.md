# Visual Guide: The Circular Dependency Problem

## Current Architecture (Why It Fails)

```
┌─────────────────────────────────────────────────────────────┐
│                      synthfs package                         │
│                                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Operation Interface                     │    │
│  │                    (God Object)                      │    │
│  │  • ID()           • Execute()    • GetItem()        │    │
│  │  • Validate()     • Rollback()   • Dependencies()   │    │
│  │  • ReverseOps()   • Describe()   • GetChecksum()    │    │
│  └──────────────────────┬──────────────────────────────┘    │
│                         │                                    │
│          ┌──────────────┼──────────────┬─────────────┐      │
│          │              │              │             │      │
│          ▼              ▼              ▼             ▼      │
│     ┌─────────┐   ┌──────────┐   ┌─────────┐  ┌─────────┐ │
│     │ Batch   │   │ Executor │   │Pipeline │  │Operations│ │
│     │         │   │          │   │         │  │ (impls)  │ │
│     │ creates │   │   runs   │   │ manages │  │          │ │
│     │  ops    │   │   ops    │   │  ops    │  │ SimpleOp │ │
│     └─────────┘   └──────────┘   └─────────┘  └─────────┘ │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## When We Try to Extract Operations

```
┌─────────────────────┐         ┌─────────────────────┐
│   synthfs package   │         │ operations package  │
│                     │         │                     │
│ • Operation interface│ ───────►│ • SimpleOperation   │
│ • Batch (creates ops)│         │   implements        │
│ • Types (ID, etc)   │◄─────── │   Operation         │
│                     │ imports │ • Needs types       │
└─────────────────────┘         └─────────────────────┘
        ▲                                │
        │            CIRCULAR!           │
        └────────────────────────────────┘
```

## The Coupling Web

```
                    ┌──────────────┐
                    │   Operation   │
                    │   Interface   │
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
   needs types        needs items        needs filesystem
        │                  │                  │
        ▼                  ▼                  ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ OperationID  │   │   FsItem     │   │  FileSystem  │
│ OperationDesc│   │  FileItem    │   │              │
│ BackupData   │   │  DirItem     │   │              │
└──────────────┘   └──────────────┘   └──────────────┘
        ▲                  ▲                  ▲
        │                  │                  │
        └──────────────────┴──────────────────┘
                    all defined in or
                    imported by synthfs
```

## The Smell: Feature Envy

Each component "envies" features from other components:

```
Batch says:           "I need to create operations"
                     "I need to track path states"
                     "I need to validate operations"
                     
Executor says:        "I need to run operations"
                     "I need to handle rollbacks"
                     "I need to track results"
                     
Operations say:       "I need filesystem access"
                     "I need to know about items"
                     "I need validation logic"
                     
Pipeline says:        "I need to sort operations"
                     "I need to validate deps"
                     "I need to check conflicts"
```

Everyone needs everything!

## A Better Design (Conceptual)

```
┌─────────────────────────────────────────────────────────┐
│                    core package                          │
│  • OperationID                                           │
│  • Basic types                                           │
│  • Simple interfaces                                     │
└─────────────────────────────────────────────────────────┘
                            ▲
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        │                   │                   │
┌───────▼────────┐ ┌────────▼────────┐ ┌───────▼────────┐
│   metadata     │ │   execution     │ │  operations    │
│   package      │ │   package       │ │   package      │
│                │ │                 │ │                │
│ • Describe()   │ │ • Execute()     │ │ • Concrete     │
│ • Dependencies │ │ • Validate()    │ │   operations   │
│ • Conflicts    │ │ • Rollback()    │ │                │
└────────────────┘ └─────────────────┘ └────────────────┘
```

## The Root Cause

The `Operation` interface is trying to be:
1. **A data structure** (has ID, dependencies, item)
2. **A behavior** (can execute, validate, rollback)
3. **A metadata container** (can describe itself)
4. **A relationship manager** (knows conflicts/dependencies)
5. **A advanced feature provider** (can create reverse ops)

This violates the **Single Responsibility Principle** - the interface has too many reasons to change.

## Signs This is a Problem

1. **Can't extract packages** - Circular dependencies everywhere
2. **Tests are complex** - Need to mock entire operation interface
3. **Adding features is hard** - Changes ripple everywhere
4. **Code duplication** - Similar logic in multiple places
5. **High coupling** - Change one thing, break many things

## Summary

The current architecture has evolved into a **big ball of mud** where everything depends on everything through the central `Operation` interface. This is preventing modular package extraction and making the codebase harder to maintain and extend.