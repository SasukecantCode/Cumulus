# Cumulus Backend Architecture

## Purpose

Cumulus is a cloud storage orchestration platform. The backend is responsible for authentication, account linking, storage scheduling, upload orchestration, and metadata persistence. Google Drive is only a binary storage provider; PostgreSQL remains the source of truth.

## Current Scope

- Google OAuth 2.0 authentication
- Multiple Google accounts per user
- Fill First scheduling
- File upload orchestration
- PostgreSQL-backed metadata

## Proposed Folder Structure

```text
cmd/
  cumulus/
    main.go

internal/
  config/
  domain/
  infrastructure/
    database/
      postgres/
    google/
    scheduler/
    storage/
  usecase/
  transport/
    http/
      handlers/
      middleware/
      routes/
```

## Domain Model

- User: platform identity and ownership anchor.
- GoogleAccount: one authenticated Google account with its own refresh token and storage quota snapshot.
- File: metadata for each uploaded object.
- Folder: reserved for future hierarchy support.
- StoragePolicy: reserved for future per-user policy and routing logic.

## Service Boundaries

- AuthService: OAuth URL generation, callback handling, token exchange, refresh lifecycle.
- SchedulerService: chooses the destination account for a file upload.
- StorageService: coordinates checksum, scheduling, provider upload, and metadata persistence.
- MetadataService: owns all persistence operations behind repository interfaces.

## Upload Pipeline

1. Receive upload request.
2. Compute checksum and inspect file metadata.
3. Ask the scheduler which connected Google account should receive the file.
4. Upload binary content to Google Drive.
5. Persist file metadata in PostgreSQL.
6. Return the platform file identifier and provider file identifier.

## Authentication Pipeline

1. User starts Google connection.
2. Backend redirects to Google OAuth consent.
3. Google returns an authorization code.
4. Backend exchanges the code for access and refresh tokens.
5. Backend stores the refresh token and account metadata locally.
6. Periodic refresh uses the refresh token to renew access without user interaction.

## Multi-Account Management

Each Google account is linked to exactly one user and stored independently. The backend treats each account as a distinct quota-bearing resource with its own tokens, storage snapshot, and upload eligibility. A future quota monitor can periodically refresh used and remaining capacity.

## Scheduling

The scheduler is an interface. The first implementation is Fill First, which selects the most utilized eligible account first until it reaches capacity, then moves to the next eligible account. This keeps the strategy swappable for later algorithms such as least used, round robin, or user selected.

## Architectural Improvements To Keep In Mind

- Add provider-specific interfaces per cloud vendor before introducing OneDrive or Dropbox.
- Keep OAuth and storage provider concerns isolated from HTTP handlers.
- Add transactions around file metadata and provider upload outcomes.
- Introduce domain events for quota refresh and future search/indexing.
- Store encrypted tokens at rest before production deployment.
```
