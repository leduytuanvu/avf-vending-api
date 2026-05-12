# Full Test Inventory

- Branch: `security/goose-otel-fix`
- Commit: `35e527857dfc2f42da41de9ab7e593728cacbbdb`
- Generated: `2026-05-12T08:33:11.984958+00:00`
- Counts by surface: `{'REST': 365, 'gRPC': 85, 'MQTT': 12, 'Business flow': 25}`
- Counts by status: `{'pass': 7, 'blocked-missing-seed': 368, 'blocked-provider': 11, 'blocked-hardware': 13, 'partial': 84, 'blocked': 4}`

| Surface | ID | Priority | Class | Status | Evidence/Gaps |
|---|---|---|---|---|---|
| REST | `DocOpHealthLive` | P0 | safe-read | **pass** | HTTP evidence captured |
| REST | `DocOpHealthReady` | P0 | safe-read | **pass** | HTTP evidence captured |
| REST | `DocOpMetrics` | P2 | safe-read | **pass** | HTTP evidence captured |
| REST | `DocOpSwaggerDocJSON` | P2 | safe-read | **pass** | HTTP evidence captured |
| REST | `DocOpSwaggerIndex` | P2 | safe-read | **pass** | HTTP evidence captured |
| REST | `DocOpV1AdminAssignmentsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminAuditEventsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminAuthUsersList` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminAuthUsersCreate` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminAuthUsersGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersPatch` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersActivate` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersDeactivate` | P0 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersResetPassword` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersRevokeSessions` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersPatchRoles` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersPostRoles` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersPutRoles` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersSessions` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminAuthUsersPatchStatus` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminBrandsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminBrandCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminBrandDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminBrandPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminBrandReplace` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCategoriesList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminCategoryCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminCategoryDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCategoryPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCategoryReplace` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommandsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminFeatureFlagsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminFeatureFlagsPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminFeatureFlagGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminFeatureFlagPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminFeatureFlagDisablePost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminFeatureFlagEnablePost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminFeatureFlagTargetsPut` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminFinanceDailyCloseList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminFinanceDailyClosePost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminFinanceDailyCloseGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminInventoryLowStock` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminInventoryRefillSuggestions` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminMachineConfigRolloutsList` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminMachineConfigRolloutsPost` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminMachineConfigRolloutGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachinesList` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminMachineCreate` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminMachineGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachinePatch` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineActivationCodesList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineActivationCodesPost` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineActivationCodeDelete` | P0 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineCashCollectionsList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineCashCollectionsPost` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineCashCollectionGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineCashCollectionClosePost` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineCashbox` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineDiagnosticBundlesList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineDiagnosticRequest` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineDisable` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineEnable` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineInventory` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineInventoryEvents` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachinePlanogramDraftPut` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachinePlanogramPublishPost` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineRefillSuggestions` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineRetire` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineRotateCredential` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineSlots` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineStockAdjustmentsPost` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineSetupSyncPost` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMachineTopologyPut` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMediaList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminMediaAssetsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminMediaAssetsCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminMediaAssetsDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMediaAssetsGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMediaUploadInit` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminMediaDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMediaGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminMediaUploadComplete` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOutboxOpsGet` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminOutboxRetry` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminRetentionOpsGet` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminArtifactsList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminArtifactsReserve` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminArtifactsDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminArtifactsGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminArtifactsPutContent` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminArtifactsDownloadURL` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgActivationCodesList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgActivationCodesPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgActivationCodeRevoke` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationalAnomaliesList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationalAnomalyGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationalAnomalyIgnore` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationalAnomalyResolve` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgAssignmentsList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgAssignmentsCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgAssignmentDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgAssignmentGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationAuditEventsList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationAuditEventGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsCommandsList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsCommandGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsCommandCancel` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsCommandRetry` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommerceReconciliationList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommerceReconciliationGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommerceReconciliationIgnore` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommerceReconciliationResolve` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsInventoryAnomaliesList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsInventoryAnomalyResolve` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachinesList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachinesCreate` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachinePatch` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineArchive` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsMachineCommandsDispatch` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineOperationalHealthGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsMachineInventoryAnomaliesList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsMachineInventoryReconcile` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineMarkCompromised` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineResume` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineRevokeCredentials` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineRevokeToken` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineRotateCredentials` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineRotateTokenVersion` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineSuspend` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineTechniciansList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineTechniciansCreate` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineTechnicianDelete` | P0 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineOperationalTimeline` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgMachineTransferSite` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationMediaAssetsList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationMediaAssetsDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationMediaAssetsGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationMediaProductImagesPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationMediaUploadComplete` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationMediaUploadInit` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgOperationsMachinesHealthList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommerceOrderRefundPost` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommerceOrderTimelineGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationProductImagesList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationProductImagesPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationProductImagesDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationProductImagesPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationProductMediaPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrganizationProductMediaDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgProvisioningBatchGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgProvisioningBulkCreate` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommerceRefundRequestsList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminCommerceRefundRequestGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsCash` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsCommands` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsExport` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsFailedVends` | P0 | hardware-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsTechnicianFills` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsInventoryUnified` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsInventoryLowStock` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsMachineHealth` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsMachines` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsPayments` | P0 | provider-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsProducts` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsReconciliationBI` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsReconciliationQueue` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsRefunds` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsSales` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgReportsVends` | P0 | hardware-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRestockSuggestions` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRolloutsList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRolloutsCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRolloutsGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRolloutsCancel` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRolloutsPause` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRolloutsResume` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRolloutsRollback` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgRolloutsStart` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgSitesList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgSitesCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgSiteDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgSiteGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgSitePatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgSiteArchive` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgTechniciansList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgTechniciansCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgTechnicianGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgTechnicianPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgTechnicianDisable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgTechnicianEnable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersList` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersDisable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersEnable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersResetPassword` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersRevokeSessions` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersPatchRoles` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersRoles` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersDeleteRole` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersSessions` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOrgUsersPatchStatus` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTAList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminOTACampaignsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminOTACampaignCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminOTACampaignGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignApprove` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignCancel` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignPause` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignPublish` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignResultsGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignResume` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignRollback` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignStart` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignTargetsGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminOTACampaignTargetsPut` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPlanogramsList` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminPlanogramGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBooksList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminPriceBookCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminPriceBookGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookActivate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookArchive` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookAssignTarget` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookDeactivate` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookItemsGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookItemsPut` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookItemDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookItemPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPriceBookTargetDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPricingPreview` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminProductsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminProductCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminProductDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductReplace` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductImageDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductImagePost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductImagePut` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductMediaPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductMediaPut` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminProductMediaDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPromotionsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminPromotionCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminPromotionsPreview` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminPromotionGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPromotionPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPromotionActivate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPromotionArchive` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPromotionAssignTarget` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPromotionDeactivate` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPromotionPause` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminPromotionTargetDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminReportsCashCollectionsExportCSV` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminReportsPaymentsSummaryExportCSV` | P0 | provider-required | **blocked-provider** | blocked-provider: external dependency required |
| REST | `DocOpV1AdminReportsSalesSummaryExportCSV` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminSitesList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminSiteCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminSiteDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminSiteGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminSitePatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminSiteDisable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminSystemOutboxListGet` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminSystemOutboxStatsGet` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminSystemOutboxGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminSystemOutboxMarkDLQPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminSystemOutboxReplayPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminSystemRetentionDryRunPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminSystemRetentionRunPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminSystemRetentionStatsGet` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminTagsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminTagCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminTagDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTagPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTagReplace` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTechnicianAssignmentsList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminTechnicianAssignmentCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminTechnicianAssignmentDelete` | P2 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTechnicianAssignmentGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTechnicianAssignmentPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTechnicianAssignmentCancel` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTechniciansList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminTechnicianCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminTechnicianGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTechnicianPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTechnicianDisable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminTechnicianEnable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AdminUsersCreate` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AdminUsersGet` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersPatch` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersDisable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersEnable` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersResetPassword` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersRevokeSessions` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersPatchRoles` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersPostRoles` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersPutRoles` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersSessions` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AdminUsersPatchStatus` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1AuthChangePassword` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthLogin` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthLogout` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthMe` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AuthMFADisable` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthMFAEnroll` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthMFAVerify` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthPasswordChange` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthPasswordResetConfirm` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthPasswordResetRequest` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthRefresh` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1AuthSessionsRevokeOthers` | P0 | destructive | **blocked-hardware** | blocked-hardware: destructive route requires explicit scenario guard |
| REST | `DocOpV1AuthSessionsList` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1AuthSessionDelete` | P0 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceCashCheckout` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1CommerceCreateOrder` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1CommerceGetOrder` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceOrderCancel` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommercePaymentSession` | P0 | provider-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommercePaymentWebhook` | P0 | provider-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceReconciliationSnapshot` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceRefundsList` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceRefundCreate` | P0 | destructive | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceRefundGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceVendFailure` | P0 | hardware-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceVendStart` | P0 | hardware-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1CommerceVendSuccess` | P0 | hardware-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1DeviceMachineCommandsPoll` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1DeviceTelemetryReconcileBatch` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1DeviceTelemetryReconcileStatusGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1DeviceMachineVendResults` | P0 | hardware-required | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineCheckIn` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineCommandDispatch` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineCommandReceipts` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineCommandStatus` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineConfigApply` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorSessionActionAttributions` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorSessionAuthEvents` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorSessionCurrent` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorSessionHistory` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorSessionLogin` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorSessionLogout` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorSessionTimeline` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorSessionHeartbeat` | P0 | local-write | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineSaleCatalogGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineShadowGet` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineTelemetryIncidents` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineTelemetryRollups` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1MachineTelemetrySnapshot` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorInsightsTechnicianAttributions` | P2 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpV1OperatorInsightsUserAttributions` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1OrdersList` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1PaymentsList` | P0 | provider-required | **blocked-provider** | blocked-provider: external dependency required |
| REST | `DocOpV1ReportsFleetHealth` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1ReportsInventoryExceptions` | P1 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1ReportsPaymentsSummary` | P0 | provider-required | **blocked-provider** | blocked-provider: external dependency required |
| REST | `DocOpV1ReportsSalesSummary` | P2 | safe-read | **blocked-missing-seed** | auth/role credentials required |
| REST | `DocOpV1SetupActivationClaimPost` | P2 | local-write | **blocked-missing-seed** | blocked-missing-seed: generic local write needs scenario-specific fixtures; covered by E2E scripts |
| REST | `DocOpV1SetupMachineBootstrap` | P1 | safe-read | **blocked-missing-seed** | blocked-missing-seed: templated path requires seeded resource IDs |
| REST | `DocOpVersion` | P0 | safe-read | **pass** | HTTP evidence captured |
| gRPC | `GetSaleCatalogSnapshot` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetOrderPaymentVendState` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| gRPC | `GetMachineSlotInventory` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMachineSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMachineState` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMachineCabinetSlotSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetLatestMachineTelemetry` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMachineIncidentSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetPaymentById` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| gRPC | `GetLatestPaymentForOrder` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| gRPC | `GetSalesSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `ActivateMachine` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `ClaimActivation` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `RefreshMachineToken` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetBootstrap` | P0 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `CheckIn` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `AckConfigVersion` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `CheckForUpdates` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `GetSaleCatalog` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SyncSaleCatalog` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `GetCatalogSnapshot` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetCatalogDelta` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `AckCatalogVersion` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `GetMediaManifest` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetPendingCommands` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `AckCommand` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `RejectCommand` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `GetAssignedUpdate` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `ReportUpdateStatus` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `ReportDiagnosticBundleResult` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `CreateOrder` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `CreatePaymentSession` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| gRPC | `AttachPaymentResult` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| gRPC | `ConfirmCashPayment` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| gRPC | `CreateCashCheckout` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `GetOrder` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetOrderStatus` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `StartVend` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `ConfirmVendSuccess` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `ReportVendSuccess` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `ReportVendFailure` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `CancelOrder` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `CreateSale` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `AttachPayment` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| gRPC | `ConfirmCashReceived` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `StartVend` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `CompleteVend` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `FailVend` | P0 | hardware-required | **blocked-hardware** | requires canary hardware/device ACK evidence |
| gRPC | `CancelSale` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `PushInventoryDelta` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetInventorySnapshot` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `AckInventorySync` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `GetPlanogram` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitStockSnapshot` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitFillResult` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitFillReport` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitRestock` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitInventoryAdjustment` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitStockAdjustment` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `ClaimActivation` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `RefreshMachineToken` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMediaManifest` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMediaDelta` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `AckMediaVersion` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `PushOfflineEvents` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetSyncCursor` | P0 | write | **blocked-missing-seed** | MACHINE_TOKEN not set for authenticated machine write method |
| gRPC | `OpenOperatorSession` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `CloseOperatorSession` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitFillReport` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitStockAdjustment` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `LoginOperator` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `LogoutOperator` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `HeartbeatOperatorSession` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `PushTelemetryBatch` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `PushCriticalEvent` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `CheckIn` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `SubmitTelemetryBatch` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `ReconcileEvents` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetEventStatus` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMachineSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMachineState` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMachineCabinetSlotSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetLatestMachineTelemetry` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetMachineIncidentSummary` | P1 | read-only | **partial** | server reachable; generic grpcurl call requires method-specific request template |
| gRPC | `GetOrderPaymentVendState` | P0 | provider-required | **blocked-provider** | requires payment/provider sandbox evidence |
| MQTT | `connect` | P0 | safe-read | **pass** | publish/subscribe round-trip evidence captured |
| MQTT | `telemetry_publish` | P0 | local-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| MQTT | `command_subscribe` | P0 | safe-read | **partial** | broker reachable; flow needs scenario assertion |
| MQTT | `command_dispatch` | P0 | canary-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| MQTT | `ack_success` | P0 | canary-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| MQTT | `ack_duplicate` | P1 | canary-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| MQTT | `ack_timeout_no_ack` | P0 | hardware-required | **blocked-hardware** | requires real canary device reconnect/no-ACK evidence |
| MQTT | `invalid_payload` | P1 | local-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| MQTT | `topic_prefix_correctness` | P0 | safe-read | **partial** | broker reachable; flow needs scenario assertion |
| MQTT | `crlf_windows_handling` | P1 | local-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| MQTT | `reconnect_offline` | P1 | hardware-required | **blocked-hardware** | requires real canary device reconnect/no-ACK evidence |
| MQTT | `acl_topic_isolation` | P1 | production-readonly | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| Business flow | `admin login/logout/session/refresh` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `invalid token/expired token` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `RBAC allow/deny` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `organization/site/user/admin CRUD` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `machine create/register/activate/deactivate` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `machine token/credential lifecycle` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `config/bootstrap/catalog/media pull` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `product/category/media CRUD` | P1 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `planogram/slot publish` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `inventory restock/adjustment/out-of-stock` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `sale catalog` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `cash sale success` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `payment/QR sale success` | P0 | provider/hardware-required | **blocked** | Requires configured provider/hardware/canary for production proof |
| Business flow | `dispense success` | P0 | provider/hardware-required | **blocked** | Requires configured provider/hardware/canary for production proof |
| Business flow | `dispense failure/refund` | P0 | provider/hardware-required | **blocked** | Requires configured provider/hardware/canary for production proof |
| Business flow | `duplicate webhook/replay` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `invalid webhook/HMAC` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `amount/currency mismatch` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `remote command dispatch/ACK/fail/timeout` | P0 | provider/hardware-required | **blocked** | Requires configured provider/hardware/canary for production proof |
| Business flow | `diagnostics create/resolve` | P1 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `telemetry ingest` | P1 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `offline replay/idempotency/outbox retry` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `reporting/audit` | P1 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `backup/restore docs/drill` | P2 | local-executable | **partial** | Covered by local harness when dependencies are running |
| Business flow | `health/live/ready/version` | P0 | local-executable | **partial** | Covered by local harness when dependencies are running |
