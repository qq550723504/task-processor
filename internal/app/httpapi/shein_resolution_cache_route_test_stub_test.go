package httpapi

import "github.com/gin-gonic/gin"

func (h *stubListingKitHandler) ClearSheinResolutionCache(c *gin.Context) {}

func (h *stubListingKitHandler) RegenerateSheinDataImage(c *gin.Context) {}

func (h *stubListingKitHandler) CreateSDSRetirementRun(c *gin.Context) {}

func (h *stubListingKitHandler) GetSDSRetirementRun(c *gin.Context) {}

func (h *stubListingKitHandler) UpdateSDSRetirementSelection(c *gin.Context) {}

func (h *stubListingKitHandler) ConfirmSDSRetirementRun(c *gin.Context) {}

func (h *stubListingKitHandler) RetrySDSRetirementRun(c *gin.Context) {}

func (h *stubListingKitHandler) ListSheinSDSCostGroups(c *gin.Context) {}

func (h *stubListingKitHandler) ListSheinSourceSDSMetadata(c *gin.Context) {}

func (h *stubListingKitHandler) UpdateSheinSDSCostGroup(c *gin.Context) {}
