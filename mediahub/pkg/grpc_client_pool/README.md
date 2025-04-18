## 优化点解析

### 优化前的问题（基于 `sync.Pool`）：
1. **连接创建时机不可控**：`sync.Pool` 更适合用于轻量对象回收，而不是连接这种昂贵资源；
2. **连接状态检测不严谨**：若 `conn` 被关闭或异常，重复 `.Get()` 可能返回已无效的连接；
3. **连接复用无序**：池中无法控制连接数量与分配策略，可能存在频繁创建、回收的开销；
4. **并发安全不完善**：虽然 `sync.Pool` 是线程安全的，但你无法控制并发中连接的分配方式；

---

### 优化后的提升点：

| 优化方向         | 实现方式                                               | 效果提升                             |
|------------------|--------------------------------------------------------|--------------------------------------|
| **连接复用策略** | 自定义固定数量连接池 `[]*grpc.ClientConn`，支持轮询复用 | 控制连接总量，避免资源爆炸            |
| **线程安全**     | 使用 `sync.Mutex` 实现连接分配加锁                    | 避免并发访问冲突，保持一致性          |
| **健康检查**     | 每次 `.Get()` 时检测连接状态（Shutdown、Failure）     | 杜绝返回无效连接，提升系统稳定性      |
| **复用轮询调度** | 基于 `currIndex` 实现**伪轮询策略**                   | 合理分发请求，提升连接使用效率        |
| **连接重建机制** | 无效连接时主动关闭并替换                               | 保证连接池中始终为可用连接            |

---