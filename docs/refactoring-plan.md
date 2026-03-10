# 项目重构计划

## 概述

基于重复目录分析报告,本文档提供详细的重构计划,包括具体步骤、预期收益和风险评估。

生成时间: 2026-03-10  
参考文档: `docs/duplicate-directories-analysis.md`

---

## 重构原则

1. **小步快跑**: 每次只重构一个问题,立即提交
2. **测试先行**: 重构前确保有测试覆盖,重构后运行测试
3. **向后兼容**: 尽量保持API兼容性
4. **渐进式**: 优先解决高优先级问题
5. **可回滚**: 每次提交都应该是可独立回滚的

---

## 阶段一: 命名规范统一 (高优先级)

### 任务 1.1: 统一 TEMU 平台的 service 命名

**问题描述:**
TEMU平台存在三个service相关目录,命名不一致:
- `internal/platforms/temu/service`
- `internal/platforms/temu/services`
- `internal/platforms/temu/api/services`

**重构目标:**
统一为单数形式 `service`,并明确各目录职责

**具体步骤:**

#### 步骤 1: 分析现有代码结构
```bash
# 列出各目录下的文件
ls internal/platforms/temu/service
ls internal/platforms/temu/services
ls internal/platforms/temu/api/services
```

#### 步骤 2: 确定重构方案
- 保留 `internal/platforms/temu/services` 作为主服务层
- 将 `internal/platforms/temu/service/scheduler` 移动到 `services/scheduler`
- 将 `internal/platforms/temu/api/services` 重命名为 `api/endpoints` 或合并到 `api`

#### 步骤 3: 执行重构
1. 使用 `smartRelocate` 移动文件(自动更新引用)
2. 更新包名和导入路径
3. 运行测试确保功能正常
4. 提交代码

**预期收益:**
- 消除命名混乱
- 提高代码可读性
- 统一项目命名规范

**风险评估:**
- 风险等级: 中
- 影响范围: TEMU平台相关代码
- 缓解措施: 使用 `smartRelocate` 自动更新引用

**预计工作量:** 2-3小时

---

### 任务 1.2: 重命名 Amazon 的 internal 目录

**问题描述:**
`internal/platforms/amazon/internal` 嵌套命名不清晰

**重构目标:**
重命名为更具体的名称,如 `core` 或 `impl`

**具体步骤:**

#### 步骤 1: 分析目录内容
```bash
# 查看目录内容
ls internal/platforms/amazon/internal
```

#### 步骤 2: 确定新名称
根据目录内容选择合适的名称:
- 如果是核心实现 → `core`
- 如果是内部实现细节 → `impl`
- 如果是私有辅助代码 → `private`

#### 步骤 3: 执行重构
1. 使用 `smartRelocate` 重命名目录
2. 更新相关文档
3. 运行测试
4. 提交代码

**预期收益:**
- 消除嵌套命名混淆
- 提高代码可读性

**风险评估:**
- 风险等级: 低
- 影响范围: Amazon平台相关代码
- 缓解措施: 使用 `smartRelocate` 自动更新引用

**预计工作量:** 1小时

---

## 阶段二: 模糊命名优化 (高优先级)

### 任务 2.1: 审查和重构 utils 目录

**问题描述:**
项目中有5个 `utils` 目录,容易成为"垃圾桶",且可能存在重复代码:
- `internal/core/config/utils`
- `internal/pkg/utils`
- `internal/platforms/amazon/utils`
- `internal/platforms/shein/utils`
- `internal/platforms/temu/utils`

**重构目标:**
1. 重命名为更具体的名称
2. 提取公共代码到 `internal/pkg/utils`
3. 确保各平台utils只包含平台特定代码

**具体步骤:**

#### 步骤 1: 审查各 utils 目录内容
```bash
# 列出所有utils目录的文件
find internal -name "utils" -type d -exec sh -c 'echo "=== {} ===" && ls {}' \;
```

#### 步骤 2: 分析代码重复
1. 识别各平台utils中的公共函数
2. 识别可以提取到 `internal/pkg/utils` 的代码
3. 识别真正平台特定的代码

#### 步骤 3: 重命名配置工具目录
```
internal/core/config/utils → internal/core/config/helpers
```

#### 步骤 4: 提取公共代码
1. 将各平台utils中的公共函数移到 `internal/pkg/utils`
2. 更新导入路径
3. 删除重复代码

#### 步骤 5: 重命名平台utils(可选)
如果平台utils有明确用途,考虑重命名:
- `amazon/utils` → `amazon/helpers` 或 `amazon/adapters`
- `shein/utils` → `shein/helpers`
- `temu/utils` → `temu/helpers`

**预期收益:**
- 消除代码重复
- 提高代码可维护性
- 更清晰的命名

**风险评估:**
- 风险等级: 中
- 影响范围: 多个模块
- 缓解措施: 分步骤执行,每步都提交

**预计工作量:** 4-6小时

---

### 任务 2.2: 审查和重构 common 目录

**问题描述:**
项目中有3个 `common` 目录,命名模糊:
- `internal/platforms/common`
- `internal/platforms/shein/service/common`
- `internal/platforms/temu/handlers/common`

**重构目标:**
审查代码,确保真正是"通用"的,或重命名为更具体的名称

**具体步骤:**

#### 步骤 1: 审查 platforms/common
1. 查看目录内容
2. 确认是否真的是所有平台通用的代码
3. 如果是,保持不变
4. 如果不是,考虑移到具体平台或重命名

#### 步骤 2: 审查 shein/service/common
1. 查看目录内容
2. 如果是SHEIN服务的基础类,考虑重命名为 `base` 或 `foundation`
3. 如果是共享工具,考虑重命名为 `shared`

#### 步骤 3: 审查 temu/handlers/common
1. 查看目录内容
2. 如果是TEMU处理器的基础类,考虑重命名为 `base`
3. 如果是共享逻辑,考虑重命名为 `shared`

**预期收益:**
- 更清晰的命名
- 避免"垃圾桶"目录

**风险评估:**
- 风险等级: 低
- 影响范围: 各平台代码
- 缓解措施: 逐个目录处理

**预计工作量:** 2-3小时

---

## 阶段三: 职责明确化 (中优先级)

### 任务 3.1: 明确各平台 scheduler 职责

**问题描述:**
TEMU和SHEIN都有 `scheduler` 和 `service/scheduler` 两层,可能存在职责重叠

**重构目标:**
明确各层职责,消除重复

**具体步骤:**

#### 步骤 1: 分析现有结构
```bash
# SHEIN
ls internal/platforms/shein/scheduler
ls internal/platforms/shein/service/scheduler

# TEMU
ls internal/platforms/temu/scheduler
ls internal/platforms/temu/service/scheduler
```

#### 步骤 2: 确定职责划分
建议划分:
- `platforms/{platform}/scheduler`: 调度器入口,任务注册和管理
- `platforms/{platform}/service/scheduler`: 具体的调度服务实现

#### 步骤 3: 重构代码
1. 如果职责重叠,合并到一个目录
2. 如果职责清晰,保持现状但添加文档说明
3. 更新相关文档

**预期收益:**
- 清晰的职责划分
- 避免代码重复

**风险评估:**
- 风险等级: 中
- 影响范围: 各平台调度相关代码
- 缓解措施: 先分析再决定是否重构

**预计工作量:** 3-4小时

---

## 阶段四: 代码提取和复用 (中优先级)

### 任务 4.1: 提取平台间公共代码

**问题描述:**
各平台可能存在重复的业务逻辑

**重构目标:**
识别并提取公共代码到 `internal/platforms/common`

**具体步骤:**

#### 步骤 1: 识别公共模式
1. 比较各平台的相似功能(如图片处理、分类处理等)
2. 识别可以抽象的公共接口
3. 识别可以复用的公共实现

#### 步骤 2: 设计公共接口
1. 在 `internal/platforms/common` 中定义接口
2. 设计可扩展的抽象层

#### 步骤 3: 提取公共实现
1. 将公共代码移到 `common`
2. 各平台实现特定接口
3. 更新导入路径

#### 步骤 4: 重构各平台代码
1. 使用公共接口和实现
2. 删除重复代码
3. 保留平台特定逻辑

**预期收益:**
- 减少代码重复
- 提高代码复用性
- 简化新平台接入

**风险评估:**
- 风险等级: 高
- 影响范围: 所有平台
- 缓解措施: 小步快跑,逐个功能提取

**预计工作量:** 8-12小时

---

## 阶段五: 文档和测试完善 (低优先级)

### 任务 5.1: 完善目录结构文档

**重构目标:**
为每个主要目录添加 README.md,说明其职责和使用方式

**具体步骤:**

#### 步骤 1: 识别需要文档的目录
- `internal/app`
- `internal/application`
- `internal/domain`
- `internal/platforms`
- 各平台子目录

#### 步骤 2: 编写 README.md
每个README应包含:
1. 目录职责说明
2. 主要组件介绍
3. 使用示例
4. 依赖关系

#### 步骤 3: 更新架构文档
更新 `docs/architecture` 中的相关文档

**预期收益:**
- 提高代码可读性
- 降低新人上手难度

**预计工作量:** 4-6小时

---

### 任务 5.2: 增加测试覆盖率

**重构目标:**
为重构后的代码增加测试覆盖

**具体步骤:**

#### 步骤 1: 识别测试缺口
```bash
# 运行测试覆盖率分析
go test -cover ./...
```

#### 步骤 2: 编写单元测试
优先为以下模块增加测试:
- 公共工具函数
- 领域模型
- 关键业务逻辑

#### 步骤 3: 编写集成测试
为平台集成功能增加测试

**预期收益:**
- 提高代码质量
- 降低重构风险

**预计工作量:** 8-12小时

---

## 执行时间表

### 第一周: 命名规范统一
- 周一-周二: 任务 1.1 (TEMU service命名)
- 周三: 任务 1.2 (Amazon internal重命名)

### 第二周: 模糊命名优化
- 周一-周三: 任务 2.1 (utils重构)
- 周四-周五: 任务 2.2 (common重构)

### 第三周: 职责明确化
- 周一-周三: 任务 3.1 (scheduler职责)

### 第四周: 代码提取和复用
- 周一-周五: 任务 4.1 (提取公共代码)

### 第五周: 文档和测试
- 周一-周三: 任务 5.1 (完善文档)
- 周四-周五: 任务 5.2 (增加测试)

---

## 成功标准

### 代码质量指标
- [ ] 消除所有命名不一致问题
- [ ] utils和common目录有明确职责
- [ ] 代码重复率降低30%以上
- [ ] 测试覆盖率达到60%以上

### 可维护性指标
- [ ] 新人能在1天内理解项目结构
- [ ] 添加新平台的时间减少50%
- [ ] 代码审查时间减少30%

### 文档完整性
- [ ] 所有主要目录都有README
- [ ] 架构文档更新完整
- [ ] 重构决策有记录

---

## 风险管理

### 高风险项
1. **任务 4.1 (提取公共代码)**: 影响范围大,需要充分测试
   - 缓解: 小步快跑,每次只提取一个功能
   - 回滚: 每次提交都可独立回滚

### 中风险项
2. **任务 1.1 (TEMU命名)**: 可能影响现有功能
   - 缓解: 使用 `smartRelocate` 自动更新引用
   - 回滚: Git revert

3. **任务 2.1 (utils重构)**: 涉及多个模块
   - 缓解: 分步骤执行,每步都测试
   - 回滚: Git revert

### 低风险项
4. **任务 5.1 (文档)**: 不影响代码功能
5. **任务 1.2 (Amazon重命名)**: 影响范围小

---

## 回滚计划

每个任务都应该:
1. 在独立的Git分支上进行
2. 完成后合并到主分支
3. 如果出现问题,可以快速回滚

回滚命令:
```bash
# 回滚最后一次提交
git revert HEAD

# 回滚到特定提交
git revert <commit-hash>

# 如果需要完全回退
git reset --hard <commit-hash>
```

---

## 总结

本重构计划采用渐进式方法,优先解决高优先级问题,逐步改善代码质量和可维护性。

预计总工作量: 30-45小时  
预计完成时间: 5周

关键成功因素:
1. 小步快跑,每次只改一个问题
2. 充分测试,确保功能正常
3. 及时提交,保持可回滚性
4. 持续沟通,及时调整计划
