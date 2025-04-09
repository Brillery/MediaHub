-- 创建名为 `mediahub` 的数据库模式，并设置默认字符集为 `utf8mb4`
CREATE SCHEMA `mediahub` DEFAULT CHARACTER SET utf8mb4 ;

-- 使用 `mediahub` 数据库
use mediahub;

-- 创建 `url_map` 表，用于存储短链接与原始URL的映射关系
CREATE TABLE `mediahub`.`url_map` (
                                      `id` BIGINT(20) NOT NULL AUTO_INCREMENT,  -- 主键，自增ID
                                      `short_key` VARCHAR(45) NOT NULL DEFAULT '',  -- 短链接的唯一标识
                                      `original_url` VARCHAR(512) NOT NULL DEFAULT '',  -- 原始URL
                                      `times` INT NOT NULL DEFAULT 0,  -- 该短链接被访问的次数
                                      `create_at` BIGINT(64) NOT NULL DEFAULT 0,  -- 记录创建时间的时间戳
                                      `update_at` BIGINT(64) NOT NULL DEFAULT 0,  -- 记录最后一次更新时间的时间戳
                                      PRIMARY KEY (`id`),  -- 设置 `id` 为主键
                                      INDEX `index_short_key` (`short_key` ASC) VISIBLE,  -- 在 `short_key` 字段上创建索引
                                      INDEX `index_original_url` (`original_url` ASC) VISIBLE)  -- 在 `original_url` 字段上创建索引
    ENGINE = InnoDB  -- 使用 InnoDB 存储引擎
DEFAULT CHARACTER SET = utf8mb4  -- 设置默认字符集为 `utf8mb4`
COMMENT = 'url关系表';  -- 表注释，说明该表用于存储URL映射关系

-- 创建 `url_map_user` 表，用于存储用户与短链接的映射关系
CREATE TABLE `mediahub`.`url_map_user` (
                                           `id` BIGINT(20) NOT NULL AUTO_INCREMENT,  -- 主键，自增ID
                                           `user_id` BIGINT(20) NOT NULL DEFAULT 0,  -- 用户ID，关联到具体用户
                                           `short_key` VARCHAR(45) NOT NULL DEFAULT '',  -- 短链接的唯一标识
                                           `original_url` VARCHAR(512) NOT NULL DEFAULT '',  -- 原始URL
                                           `times` INT NOT NULL DEFAULT 0,  -- 该短链接被访问的次数
                                           `create_at` BIGINT(64) NOT NULL DEFAULT 0,  -- 记录创建时间的时间戳
                                           `update_at` BIGINT(64) NOT NULL DEFAULT 0,  -- 记录最后一次更新时间的时间戳
                                           PRIMARY KEY (`id`),  -- 设置 `id` 为主键
                                           INDEX `index_short_key` (`short_key` ASC) VISIBLE,  -- 在 `short_key` 字段上创建索引
                                           INDEX `index_original_url` (`original_url` ASC) VISIBLE)  -- 在 `original_url` 字段上创建索引
    ENGINE = InnoDB  -- 使用 InnoDB 存储引擎
DEFAULT CHARACTER SET = utf8mb4  -- 设置默认字符集为 `utf8mb4`
COMMENT = 'url关系表';  -- 表注释，说明该表用于存储用户与URL的映射关系

/*
 ### 面试场景：SQL 表结构设计与理解

#### 面试官：你好，请问你能简单介绍一下 `mediahub` 数据库中的 `url_map` 和 `url_map_user` 表吗？

**回答：**
`mediahub` 数据库中有两个表：
- `url_map` 用于存储短链接与原始URL的映射关系。
- `url_map_user` 用于存储用户与短链接的映射关系。

#### 面试官：请详细说明一下 `url_map` 表的字段及其用途。

**回答：**
`url_map` 表包含以下字段：
- `id`: 主键，自增ID，唯一标识每条记录。
- `short_key`: 短链接的唯一标识符，最大长度为45个字符，默认为空字符串。
- `original_url`: 原始URL，最大长度为512个字符，默认为空字符串。
- `times`: 记录该短链接被访问的次数，默认为0。
- `create_at`: 记录创建时间的时间戳，默认为0。
- `update_at`: 记录最后一次更新时间的时间戳，默认为0。

#### 面试官：为什么 `original_url` 字段使用的是 `VARCHAR(512)`？你认为这个长度合适吗？

**回答：**
`original_url` 使用 `VARCHAR(512)` 是为了确保能够存储较长的URL，同时避免占用过多的存储空间。大多数URL长度不会超过512个字符，因此这个长度是合理的。如果业务需求中确实需要支持更长的URL，可以考虑将其调整为 `VARCHAR(1024)`，但需要权衡性能和存储成本。

#### 面试官：`url_map_user` 表相比 `url_map` 表多了一个 `user_id` 字段，它的作用是什么？

**回答：**
`user_id` 字段用于关联具体的用户，表示该短链接是由哪个用户生成或使用的。这使得我们可以追踪每个用户的短链接活动，便于统计和管理。

#### 面试官：`create_at` 和 `update_at` 字段使用了 `BIGINT(64)` 类型，并且默认值为0，为什么不直接使用 `TIMESTAMP` 或 `DATETIME` 类型呢？

**回答：**
使用 `BIGINT(64)` 存储时间戳有几个优点：
- **精度高**：可以存储精确到毫秒的时间戳。
- **跨平台兼容性好**：不同系统对 `TIMESTAMP` 和 `DATETIME` 的处理可能有所不同，而时间戳是通用的。
- **性能优化**：在某些情况下，整数类型的时间戳查询和索引性能更好。

默认值为0是为了表示未设置的时间，实际应用中通常会通过应用程序逻辑填充正确的时间戳。

#### 面试官：`INDEX` 在这两个表中是如何使用的？它们的作用是什么？

**回答：**
在这两个表中，分别在 `short_key` 和 `original_url` 字段上创建了索引：
- `INDEX index_short_key (short_key ASC) VISIBLE`: 加速基于 `short_key` 的查询操作。
- `INDEX index_original_url (original_url ASC) VISIBLE`: 加速基于 `original_url` 的查询操作。

索引的主要作用是提高查询效率，尤其是在表数据量较大时，索引可以显著减少查询时间。

#### 面试官：假设你需要添加一个新的字段来记录短链接的有效期，你会如何设计这个字段？

**回答：**
我会添加一个名为 `expire_at` 的字段，类型为 `BIGINT(64)`，用于存储有效期的时间戳。默认值可以设置为0，表示没有设置有效期。具体语句如下：

```sql
ALTER TABLE `mediahub`.`url_map`
ADD COLUMN `expire_at` BIGINT(64) NOT NULL DEFAULT 0 COMMENT '短链接有效期的时间戳';

ALTER TABLE `mediahub`.`url_map_user`
ADD COLUMN `expire_at` BIGINT(64) NOT NULL DEFAULT 0 COMMENT '短链接有效期的时间戳';
```


这样可以方便地通过时间戳来判断短链接是否过期。

#### 面试官：最后一个问题，如果你要优化这些表的性能，你会从哪些方面入手？

**回答：**
我会从以下几个方面进行优化：
1. **索引优化**：确保索引只创建在频繁查询的字段上，避免不必要的索引增加写入开销。
2. **字段类型优化**：根据实际需求选择合适的数据类型，例如 `VARCHAR` 的长度应尽量合理，避免浪费存储空间。
3. **分区表**：如果表数据量非常大，可以考虑使用分区表来提高查询性能。
4. **缓存机制**：对于频繁访问的数据，可以引入缓存机制（如 Redis）来减轻数据库压力。
5. **定期清理**：设置定期任务清理不再需要的历史数据，保持表的精简。

---

希望这些问题和答案能帮助你更好地理解和掌握 SQL 表的设计与优化。如果有任何疑问，欢迎继续提问！
 */