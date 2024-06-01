## ds 
### Data-Structure 
- [map, sorted map, set, sorted set, queue, skiplist](https://github.com/chen3feng/stl4go)
- https://github.com/emirpasic/gods
- [Another utils for std.](https://github.com/bradenaw/juniper)
- https://github.com/axiomhq/hyperloglog
- redis hyperloglog的密集和稀疏两种struct各自的实现？执行pfadd和pfcount两个命令的具体流程？
- [How to implement kvdb? (using hashicorp/memberlist)](https://github.com/asim/kayvee)
- https://github.com/scalalang2/golang-fifo
- [boolean indexing 广告定向索引实现](https://github.com/echoface/be_indexer)
- https://github.com/fatih/semgroup
### bitmap/bitset 
- https://github.com/RoaringBitmap/roaring
- 哪些algo是基于bitmap实现的?
- https://github.com/bits-and-blooms/bitset
### string 
- 最长公共前缀
- 有效的字母异位词
- 旋转字符串
- 字符串包含
- 字符串转换成整数
- 回文判断
- 最长回文子串
- 字符串的全排列
### string 
- 比较redis, golang, c, cpp, rust中string的实现
- 找到字符串的最长无重复字串
- 1000 万字符串，其中有些是重复的，需要把重复的全部去掉，保留没有重复的字符串。
- 给出 N 个小写英文字母串，以及 0 个询问，即询问某两个串的最长公共前级的长度是多少？
- 最长回文字串长度
- 最短回文串
- 两个字符串的最大公共子串
- 手写获取两个字符串的公共字符，返回一个字符串，需要返回的字符串不能重复
- @string match
- 比较几种“字符串比较算法” (LIS, LCS, LCP(TrieTree/二分法 + 哈希查找), LPS, ED, KMP, RK(Rabin-Karp, hash), RM(Boyer-Moore), BF, Horspool, Sunday)
- KMP
- How does strings.Index() implemented in golang using RK+FNV? Why not use KMP?
- Why does golang use RK+FNV to implement string matching? Compare RK and KMP.
- BM
- misuses: string (iterate, split, concat, convert to bytes, search, replace, compare, modify)
- concat string, compare (strings.Builder(), ...)
- How to concat string in golang? Which methods is better? Why?
### string 
- [redis SDS. struct(free, len, []buf)](https://github.com/antirez/sds)
- https://github.com/seatgeek/fuzzywuzzy
- Why not use FuzzyWuzzy? Why not to use Levenshtein?
- TF-IDF, KNN(KDTree, BallTree)
- [smetrics = string metrics.](https://github.com/xrash/smetrics)
- WagnerFischer, Ukkonen, Jaro, JaroWinkler, Soundex, Hamming 这几种distance有啥区别?
### graph 
- 图的存储结构和基本操作（建立，遍历，删除节点，添加节点）
- 最小生成树
- 拓扑排序
- @shortest-path
- Compare Shortest Path algo. NW(floyd, bellman, SPFA) (dijkstra)
- 为什么Dijkstra算法每轮递推能够保证找到一个顶点的最短路径？
- 什么是负权边和负权环？既然 floyd 和 bellman 都可以处理负权边和负权环，那为什么 floyd 算法更好呢？
- 平面内有N个点，如何快速求出距离最近的点对？
- @AVL(平衡二叉树)
- @RBT
- What's RBT? Why is RBT the most widely used binary-tree? (binary-search-tree, auto balancing)
### graph 
- https://github.com/mburst/dijkstras-algorithm
- https://github.com/muzixing/graph_algorithm
- https://github.com/daac-tools/vibrato
- viterbi, 基于DP的shortest-path
- 有向图、无向图、V表示点 E表示边
- dijkstra邻接表、逆邻接表
### Array 
- 写一个算法，把一个二维数组顺时针旋转 90 度
- 求一个数组中连续子向量的最大和
- 寻找一个数组中前 k 个最大的数 (寻找 k 值)
- 一个数组，除了一个元素外，其他元素都是相等度，求那个元素？lc137
- 数组很长的情况下 N 个有序数组求交集并集
- 两个数组，求两个数组的交集
- 两个无序数组找到他们的交集
- 二维矩阵顺时针原地旋转 90 度
- 合并两个有序数组 lc88
- 连续子数组的最大和 offer42
- 数组中值出现一次的数字
- set 的两个特征？
- 怎么实现 set？
- 手写用 map 实现 set，支持 CURD？
### List 
- [skiplist in golang. 插入/删除/搜索 都是 O(log n)，它最大的优势是原理简单、容易实现、方便扩展、效率更高。因此在一些热门的项目里用来替代平衡树，如 redis, leveldb 等。](https://github.com/huandu/skiplist)
- [skiplist](https://gist.github.com/hxhac/ac4d4e3d808f7e26d94e117d420f16d3)
- ***skiplist有哪些独特的使用场景呢？***
- skiplist 的 randomLevel 其实就是一个普通的随机算法是吗？那为什么不直接 rand.Int(1, 16) 获取区间随机值呢？
- 跳表怎么删除节点？
- [ziplist 最大的特点在于节省内存。压缩编码、紧凑存储、连续的内存块。

ziplist 可以理解为去掉了指针的 list，这么搞可以节省内存。但是实际上这种方案的使用场景是比较有限的，毕竟通常来说，是否占用太多内存并不重要。所以只针对对内存占用十分敏感的场景。也就可以理解为什么 redis 要搞出来这个数据结构了。
](https://github.com/staeter/ziplist)
- redis ziplist 连锁更新 是啥? 会导致哪些问题? 怎么避免?
- Why redis using skiplist to implement list, hash and zset, rather than RBT?
### linked-list 
- https://gist.github.com/hxhac/3de94ae15933d23d1772eaaacfe79eb4
- 分别实现单向链表、无序链表和双向链表，还有循环链表？linked-list, unordered-linked-list, double-linked-list, circular-linked-list
- 链表到底有序还是无序？
- 怎么对链表进行排序操作？
- 有一个链表，奇数位升序，偶数位降序，如何将链表变为升序？
- 如何翻转单链表（单链表逆序）？
- 现在有一个单链表，如何判断链表中出现了环？
- 两个单向链表交叉，获取首次交叉的点
- 单向链表排序
- 合并两个有序链表 (合并两个有序数组)
- 合并两个有序的单链表
- 判断链表是否成环（龟兔赛跑）？
- 双向链表的增删改查 golang 实现
- n 个有序的单链表取交集？
- 实现一个有 pop、push、max 的队列数据结构（list），要求尽可能降低时间复杂度？
- 能否聊聊redis ziplist，相比于skiplist，ziplist为什么更节省内存？
- Implementation of zset in redis? Why does redis zset use hashmap and skiplist (instead of RBT)?
- Compare ziplist and hashtable?
- Why dose mysql use BPTree instead of skiplist?
- What's linked-list? Compare linked-list and array from (read and write operations).
- **linked-list + HashMap is a very classic combination. What other algo are implemented using this combination as a data structure?**
### HashMap 
- [set](https://github.com/deckarep/golang-set)
- [lock-free hashmap and list](https://github.com/dustinxie/lockfree)
- [map如何顺序读取? 核心思路都是构建一个struct （map+线性数据结构）。只不过线性数据结构的选择不同，一个选择list，另一个选择slice。通过“线性数据结构”实现有序，读取数据时按照这个有序的字段进行读取就可以了。](https://github.com/elliotchance/orderedmap)
- [Use slice as linear in struct.](https://github.com/iancoleman/orderedmap)
- [concurrent-map. default 32 shards, better than https://github.com/cornelk/hashmap, 但是这个pkg的key不支持generics](https://github.com/orcaman/concurrent-map)
- [相比于concurrent-map，key和value都支持generics](https://github.com/alphadose/haxmap)
- [concurrent map的一种实现，仅供参考，不建议使用](https://github.com/puzpuzpuz/xsync)
- [A map which has expiring key-value pairs.](https://github.com/zekroTJA/timedmap)
- [Extendible hashing 是哈希表的一种扩展，它用于在数据库管理系统中实现动态索引。它通过动态调整哈希表结构来优化数据访问性能。这种数据结构在数据库系统中广泛用于索引大量数据，尤其是在数据量会随时间变化的情况下。对我来说，最牛逼的是 Expand hashtable without rehash，在扩容或者缩容时不需要rehash。](https://github.com/nitish6174/extendible-hashing)
## Tree 
### Tree 
- 分层遍历二叉树
- 二叉树节点的公共祖先
- 二叉树的最大深度
- 二叉树的中序遍历和层次遍历
- 二叉树的右视图
- 通过中序遍历序列和先序序列恢复二叉树
- 获取一个二叉树的最小深度
- 二叉搜索树，两个节点被交换了位置，怎么恢复？lc99
- 判断二叉树是否为平衡二叉树
- 蛇形打印二叉树 (之字层序打印二叉树)
- [297. 二叉树的序列化与反序列化](https://leetcode-cn.com/problems/serialize-and-deserialize-binary-tree/)
- [124. 二叉树中的最大路径和](https://leetcode-cn.com/problems/binary-tree-maximum-path-sum/)
- [297. 二叉树的序列化与反序列化](https://leetcode-cn.com/problems/serialize-and-deserialize-binary-tree/)
- [104. 二叉树的最大深度](https://leetcode-cn.com/problems/maximum-depth-of-binary-tree/)
- [101. 对称二叉树](https://leetcode-cn.com/problems/symmetric-tree/)
- [226. 翻转二叉树](https://leetcode-cn.com/problems/invert-binary-tree/)
- [543. 二叉树的直径](https://leetcode-cn.com/problems/diameter-of-binary-tree/)
- [257. 二叉树的所有路径](https://leetcode-cn.com/problems/binary-tree-paths/)
- [110. 平衡二叉树](https://leetcode-cn.com/problems/balanced-binary-tree/)
- [617. 合并二叉树](https://leetcode-cn.com/problems/merge-two-binary-trees/)
- [100. 相同的树](https://leetcode-cn.com/problems/same-tree/)
- [112. 路径总和](https://leetcode-cn.com/problems/path-sum/)
- [111. 二叉树的最小深度](https://leetcode-cn.com/problems/minimum-depth-of-binary-tree/)
- [236. 二叉树的最近公共祖先](https://leetcode-cn.com/problems/lowest-common-ancestor-of-a-binary-tree/)
- [222. 完全二叉树的节点个数](https://leetcode-cn.com/problems/count-complete-tree-nodes/)
- [113. 路径总和 II](https://leetcode-cn.com/problems/path-sum-ii/)
- [437. 路径总和 III](https://leetcode-cn.com/problems/path-sum-iii/)
- [129. 求根节点到叶节点数字之和](https://leetcode-cn.com/problems/sum-root-to-leaf-numbers/)
- [662. 二叉树最大宽度](https://leetcode-cn.com/problems/maximum-width-of-binary-tree/)
- [114. 二叉树展开为链表](https://leetcode-cn.com/problems/flatten-binary-tree-to-linked-list/)
- [199. 二叉树的右视图](https://leetcode-cn.com/problems/binary-tree-right-side-view/)
- [116. 填充每个节点的下一个右侧节点指针](https://leetcode-cn.com/problems/populating-next-right-pointers-in-each-node/)
- [515. 在每个树行中找最大值](https://leetcode-cn.com/problems/find-largest-value-in-each-tree-row/)
### Trie Tree 
- [trie](https://gist.github.com/hxhac/205dd7bba11b34c888f5773f7ff112c8)
- 手写 trie 树实现敏感词过滤？
- 用 hash 和堆、trie 树实现词频统计，比较一下
- [[720. 词典中最长的单词]](https://leetcode-cn.com/problems/longest-word-in-dictionary/)
- [208. 实现 Trie (前缀树)](https://leetcode-cn.com/problems/implement-trie-prefix-tree/)
- [692. 前 K 个高频单词](https://leetcode-cn.com/problems/top-k-frequent-words/)
- [421. 数组中两个数的最大异或值](https://leetcode-cn.com/problems/maximum-xor-of-two-numbers-in-an-array/)
- [212. 单词搜索 II](https://leetcode-cn.com/problems/word-search-ii/)
- 需要屏蔽 10 万个关键字，用什么算法？
### Binary Search Tree (二叉搜索树) 
- [108. 将有序数组转换为二叉搜索树](https://leetcode-cn.com/problems/convert-sorted-array-to-binary-search-tree/)
- [98. 验证二叉搜索树](https://leetcode-cn.com/problems/validate-binary-search-tree/)
- [96. 不同的二叉搜索树](https://leetcode-cn.com/problems/unique-binary-search-trees/)
- [95. 不同的二叉搜索树 II](https://leetcode-cn.com/problems/unique-binary-search-trees-ii/)
- [173. 二叉搜索树迭代器](https://leetcode-cn.com/problems/binary-search-tree-iterator/)
- [230. 二叉搜索树中第 K 小的元素](https://leetcode-cn.com/problems/kth-smallest-element-in-a-bst/)
- [99. 恢复二叉搜索树](https://leetcode-cn.com/problems/recover-binary-search-tree/)
- 二叉搜索树，两个节点被交换位置了，怎么恢复？leetcode.99
### Binary Tree 
- 字典序的第 k 小数字 lc440
- 求二叉树的最大深度
- 求二叉树的最小深度
- 求二叉树中节点的个数
- 求二叉树中叶子节点的个数
- 求二叉树中第 k 层节点的个数
- 判断二叉树是否是平衡二叉树
- 判断二叉树是否是完全二叉树
- 两个二叉树是否完全相同？
- 两个二叉树是否互为镜像？
- 翻转二叉树 (镜像二叉树) 怎么实现？
- 求两个二叉树的最低公共祖先节点？
- 二叉树的前序遍历
- 二叉树的中序遍历 (非递归实现)
- 二叉树的后序遍历
- 前序遍历和后序遍历构造二叉树
- 在二叉树中插入节点
- 输入一个二叉树和一个整数，打印出二叉树中节点值的和等于输入整数所有的路径
- 二叉树的搜索区间
- 二叉树的层次遍历
- 二叉树内两个节点的最长距离？分为三种情况，使用递归解决
- 左⼦树的最⼤深度 + 右⼦树的最⼤深度为⼆叉树的最⻓距离
- 左⼦树中的最⻓距离即为⼆叉树的最⻓距离
- 右⼦树种的最⻓距离即为⼆叉树的最⻓距离
- 判断二叉树是否是合法的二叉查找树 (BST)？
### Tree 
- https://github.com/google/btree
- btree, bptree, RBT, suffix-tree, trie-tree, binary-tree, AST
- BST(二叉查找树), bsp(Binary Space Partition Tree), bvh, kdt, quadtree, octree
- Blink-Tree, Bw-tree
- https://github.com/tidwall/btree
- [root和branch不存储数据，只存储指针地址，数据全部存储在Leaf Node，同时Leaf Node之间用双向链表链接。每个Leaf Node是三部分组成的，即前驱指针p_prev，数据data以及后继指针p_next，同时数据data是有序的，默认是升序ASC，分布在B+tree右边的键值总是大于左边的，同时从root到每个Leaf的距离是相等的，也就是访问任何一个Leaf Node需要的IO是一样的，即索引树的高度Level + 1次IO操作。我们可以将MySQL中的索引可以看成一张小表，占用磁盘空间，创建索引的过程其实就是按照索引列排序的过程，先在sort_buffer_size进行排序，如果排序的数据量大，sort_buffer_size容量不下，就需要通过临时文件来排序，最重要的是通过索引可以避免排序操作（distinct，group by，order by）。](https://github.com/roy2220/bptree)
- [后缀树的使用场景：比如域名匹配](https://github.com/golang-infrastructure/go-domain-suffix-trie)
- How to implement suffix-tree?
- What's the key feats and benefits of suffix-tree, compare with RBT?
- suffix-tree的使用场景
- [Log Structured-Merge Tree. 磁盘顺序写.](https://github.com/eileen-code4fun/LSM-Tree)
- [trie-tree(dict-tree, prefix-tree, radix-tree)](https://github.com/paritytech/trie)
- https://github.com/gammazero/radixtree
- https://github.com/beevik/prefixtree
- [Binary Expression Tree](https://github.com/alexkappa/exp)
- https://github.com/petar/GoLLRB
## Algo 
### Hash 
- https://github.com/spaolacci/murmur3
- How it works?
- Compare pros and cons respectively.
- How to design hash algo?
- https://github.com/Cyan4973/xxHash
- https://github.com/cespare/xxhash
- https://github.com/tkaitchuck/aHash
- [处理海量网页采用的文本相似判定方法。算法的主要思想是降维，将高维的特征向量映射成一个f-bit的指纹(fingerprint)，通过比较两篇文章的f-bit指纹的Hamming Distance来确定文章是否重复或者高度近似。](https://github.com/yanyiwu/simhash)
### DHT-hash 
- [kad-dht](https://github.com/libp2p/go-libp2p-kad-dht)
- [rendezvous](https://github.com/tysonmote/rendezvous)
- Compare DHT like (Consistent Hashing(hash ring), CARP(Cache Array Route Protocol), chord(Local Resilient Hashing), HRW, KAD, Rendezvous).
- How does CH achieve balance through virtual nodes?
- How to implement DFA using trie tree in golang?
### bloomfilter 
- [bloomfilter in golang, used by Milvus and Beego.](https://github.com/bits-and-blooms/bloom)
- https://github.com/RedisBloom/redisbloom-go
- [比 hugh2632/bloomfilter 的实现更好](https://github.com/alovn/go-bloomfilter)


<details>
<summary>cuckoo为啥比bloomfilter更好?</summary>

URL: https://github.com/linvon/cuckoo-filter

相比于 BF，CF 支持动态删除元素，更快的查找性能，空间开销更小。并且比其他过滤器方案（比如说 quotient filter）要更好实现。

</details>

- [cuckoo-filter](https://github.com/seiflotfy/cuckoofilter)
- Implement BloomFilter (bitmap+)
- BF 有哪些优缺点? (FPR)
- BF 的查询和写入流程？
### backoff 
- [可以理解为某种interval的TCP window的AIMD。具体来说就是，指数退避算法会利用抖动（随机延迟）来防止连续的冲突，每次间隔的时间都是指数上升，以避免同一时间有很多请求在重试可能会造成无意义的请求。](https://github.com/cenkalti/backoff)
- [怎么获取当前attempts](https://github.com/avast/retry-go)
### FTS 
- [How to implement FTS using inverted index? Pretty good code.](https://github.com/akrylysov/simplefts)
- [FTS inspired by Apache Lucene and written in Rust.](https://github.com/quickwit-oss/tantivy)
### Page-Replacement-Algo 
- [LRU/LFU](https://github.com/hashicorp/golang-lru)
- [ARC = Adaptive Replacement Cache](https://github.com/moovweb/go-cache)
- https://github.com/ghulamghousdev/Clock-Replacement-Algorithm
- LFUDA = LFU Dynamic Aging
- Approximated LRU (second chance)
### Compression-Algo 
- https://github.com/klauspost/compress
- https://github.com/facebook/zstd
- https://github.com/lz4/lz4
- https://github.com/mcmilk/7-Zip-zstd
- 为啥不同压缩算法的通用性比较差，需要针对各种场景来进行优化?
- Compare compression algo (deflate, brotil, compress, snappy, gzip, lz4).
### Distributed-Consensus-Algo 
### raft 
- (Leader, Follower, Candidate), term, Raft 中服务器之间所有类型的通信通过两个 RPC 调用(RequestVote 用于选举, AppendEntries(一致性检查) 用于复制 log 和发送心跳)
- raft 算法中`leader 选举`和`日志复制`的流程？
- raft 算法怎么保证`安全性`？
- raft 算法的`Figure8 问题`是什么？
- 通过raft日志的index和term来确保该日志唯一
- https://github.com/hashicorp/raft
- https://github.com/hashicorp/raft-wal
- [raft是separate storage设计，更灵活，可以在不同使用场景下使用各自最优的storage](https://github.com/hashicorp/raft-boltdb)
### gossip 
- [memberlist, gossip based membership and failure detection. 使用 gossip 协议来管理集群成员资格和节点故障检测。memberlist 通过在集群中的节点之间传播成员信息和故障报告，来维护集群的成员列表和状态。memberlist 通过引入一些额外的机制（如 Lifeguard 扩展），来提高协议在处理慢速消息处理、网络延迟或丢包等问题时的鲁棒性。](https://github.com/hashicorp/memberlist)
- https://github.com/libopenstorage/gossip
- How it works?
- gossip中的Anti-Entropy和Rumor-Mongering有啥区别?
- gossip相比于raft，更适用于哪些场景? 性能、EC、弹性
### TimeWheel 
- [最小堆实现的5级时间轮](https://github.com/antlabs/timer)
### TopK 
- https://github.com/DinghaoLI/TopK-URL
- https://github.com/axiomhq/topkapi
- https://github.com/segmentio/topk
- 10 亿的数据，每个文件 1000 万行，共 100 个文件，找出前 1 万大
- 求一个数组的中位数？用快速中位数算法或者最小堆
- 从 1000 万个数字中选择出 1000 最大的数。3 种方法，排序，再选前 1000 个，堆，快排
- 从海量数据 (int 数据类型) 中找到前 50 的数字
- 亿级数据里查找相同的字符以及出现次数
- 给你一个 1T 的文件，里面都是 IP 字段，需要对这个文件基于 IP 地址从小到大进行排序？
- 一个文本文件，大约有一万行，每行一个词，要求统计出其中最频繁出现的前 10 个词，请给出思想，给出时间复杂度分析。
- 给出 N 个单词组成的熟词表，以及一篇全用小写英文书写的文章，请你按最早出现的顺序写出所有不在熟词表中的生词。
- 手写 TopK 算法：从 100 万条字符串中，找出出现频率最高的 10 条？（比如在搜索引擎中，统计搜索最热门的 10 个查询词；在歌曲库中统计下载最高的前 10 首歌等）
- 100GB url 文件，使用 1GB 内存计算出出现次数 top100 的 url 和出现的次数？尽量减少 sys call 和 IO。尽力使用完已给资源。
- 搜索引擎会通过日志文件把用户每次检索使用的字符串记录下来，每个查询串的长度为 1~255B。假设目前有 1000 万个记录（这些查询串的复杂度比较高，虽然总数是 1000 万，但如果出去重复后，那么不超过 300 万个。一个查询串的复杂度越高，说明查询它的用户越多，也就是越热门的 10 个查询串，要求使用的内存不能超过 1GB
- 有 10 个文件，每个文件 1GB，每个文件的每一行存放的都是用户的 query，每个文件的 query 都可能重复。按照 query 的频度排序。
- 有一个 1GB 大小的文件，里面的每一行是一个词，词的大小不超过 16 个字节，内存限制大小是 1MB。返回频数最高的 100 个词。
- 提取某日访问网站次数最多的那个 IP。
- 10 亿个整数找出重复次数最多的 100 个整数。
- 搜索的输入信息是一个字符串，统计 300 万条输入信息中最热门的前 10 条，每次输入的一个字符串为不超过 255B，内存使用只有 1GB。
- 有 1000 万个身份证号以及他们对应的数据，身份证号可能重复，找出出现次数最多的身份证号。
### Reliability-Algo 
- [Extract content from HTML. 可以理解为对mozilla readability算法的实现，或者说某种之前青南的那种GeneralNewsExtractor.](https://github.com/go-shiori/go-readability)
- [基本原理就是通过遍历Dom之后，通过正则提取其中的内容，对不同标签有不同加权和降权，然后把div标签替换为p标签，重新遍历，计分。最后根据分值，重新拼接内容。Mozilla Ecosystem for readability, Fathom vs Mercury vs Readability.](https://github.com/mozilla/readability)
- https://github.com/go-kratos/aegis
- [GNE](https://github.com/GeneralNewsExtractor/GeneralNewsExtractor)
- [类似GNE. To extract main article from given URL with Node.js.](https://github.com/extractus/article-extractor)
### OSRM 
- [OSRM](https://github.com/Project-OSRM/osrm-backend)
- [输入GeoJSON格式的地理数据，通过构建网络拓扑结构和路径算法来计算两个地理位置之间的最短距离。用来实现导航、路线规划和地理空间分析等应用非常有用。根据您提供的起始点、目标点和地理数据，geojson-path-finder会会决定具体使用Dijkstra算法还是A*算法来计算shortest-path](https://github.com/perliedman/geojson-path-finder)
### 多线程轮流打印问题 
- 假设有 4 个协程，分别编号为 1/2/3/4，每秒钟会有一个协程打印出自己的编号，现在要求输出编号总是按照 1/2/3/4 这样的顺序打印，共打印 100 次
- 编写一个程序，开启 3 个线程，这 3 个线程的 ID 分别为 A、B、C，每个线程将自己的 ID 在屏幕上打印 10 遍，要求输出结果必须按 ABC 的顺序显示；如：ABCABC...依次递推
- 轮流打印 dog pig cat，共打印 10 次？
- 两个人 bob 和 annie，互相喊对方的名字 10 次后，最终 bob 对 annie 说 bye bye？
- 10 个线程依次打印 1-10,11-20 和到 100？
- 三个线程交替打印至 100：线程 1 打印 1、4、7，线程 2 打印 2、5、8，线程 3 打印 3、6、9，一直打印到 100 结束
- 如何让 10 个线程按照顺序打印 0123456789？
- 怎么开 10 个线程，每个线程打印 1000 个数字，要按照顺序从 1 打印到 1w？
- 用五个线程，顺序打印数字 1~无穷大，其中每 5 个数字为 1 组，如下：其中 id 代表线程的 id
- 使用两个协程交替打印序列，一个协程打印数字，另一个协程打印字母，最终效果如下`1A2B...26Z`
- 用三个线程，顺序打印字母 A-Z，输出结果是 1A 2B 3C 1D 2E...打印完毕最后输出一个 OK
- ~~实现两个协程，其中一个产生随机数并写入到 chan 中，另一个从 chan 中读取，并打印出来，最终输出 5 个随机数~~
- 给一个数组，并发交替打印奇数和偶数，请分别用 chan、sync 和原子操作实现？
- [golang channel交替打印数字和字母，比如 1A2B...26Z](https://go.dev/play/p/MbuQWq-kwl8)
- [4个goroutine轮流打印1/2/3/4，共打印100次](https://go.dev/play/p/qbZsYAKlj07)
### sort 
- [@quick-sort](https://gist.github.com/hxhac/7e1b177bab0528d6f0a87717cd136be7)
- 为什么快排是最常用的排序算法？和堆排和归并排序进行比较？三种排序算法分别的适用场景？
- (comparison sorts, non-comparison sorts)
- Compare quicksort, heapsort, mergesort
- How to implement quicksort?
- 说下快排的大概的操作过程？快排的平均的时间复杂度？快排什么情况下是最慢的？
- heapsort 堆排序
- [sort (bubble, selection)](https://gist.github.com/hxhac/5f5ee18ac40dfd1efcf180d98c3b7c0b)
- [merge sort](https://gist.github.com/hxhac/30c9b8883d764f52e34b0ab883a2b141)
- [快排 quicksort 实现TopK](https://go.dev/play/p/d91W5fi31zo)
- [heap sort实现TopK](https://go.dev/play/p/hYMtKaeP84f)
- [插入排序](https://go.dev/play/p/Sxa1ovXRnYt)
### 并发问题 
- [Dining Philosophers Problem](https://go.dev/play/p/oqBoIin8Y5l)
- Banker's Algo, deadlock
### Encryption 
- Compare Symmetric and Asymmetric Key Encryption?
- @symmetric-encryption
- Compare symmetric encryption (DES, 3DES, AES, Blowfish, RC4, RC5, RC6)?
- types (stream ciphers, block ciphers)
- block ciphers (ECB, CBC, CFB, ...)
- How to implement a symmetric encryption??? (XOR)
- @base64
- How to implement base64?
- base64的原理？（base64怎么保证数据的完整性？）
- Why is base64 encoding needed? Why can base64 ensure data integrity?
- @RSA
- How to implement RSA??? (Diffie-Hellman)
- ***What is the process of RSA+AES hybrid encryption?***
### golang笔试题 
- 怎么使用 golang 实现 java 的迭代器？想要支持所有 golang 类型，怎么封装
- timewheel
- 怎么实现 gin 的路由算法
- 怎么实现 gin 的中间件


<details>
<summary>怎么使用 golang 实现 wrr 算法？加权轮询算法(wrr). 几种实现方法 random, RBT, LVS, nginx</summary>

URL: https://github.com/wenj91/wrr

Doc: https://mp.weixin.qq.com/s?__biz=MzA4MTc4NTUxNQ==&mid=2650525165&idx=1&sn=136bf923194c8dea95f603c7b83baf57

raft 和 wrr 感觉很像
都是对 nodes 中多 node 的调度，wrr 有随机、LVS、nginx 好几种调度策略，实际上我现在这个 raft 里的选举不就是直接随机了吗？


</details>

- 怎么自己实现阻塞读并且并发安全的 map


<details>
<summary>手写 retry 方法（retry 里面的 func）？核心是什么？</summary>

URL: https://go.dev/play/p/UTRnHf9zAjW

Doc: https://medium.com/@wojtekplatek529/go-worker-pool-with-retry-and-timeout-0e072acf8726

设计Retry函数，三个参数嘛 max_attempts, func, sleep_time.

</details>

- 用 golang 手写 redis 的 pub/sub？至少给出伪代码
- 写一个 golang 爬虫，自动伸缩，自适应流控，也就是说在对方限流时就减少并发数，没有限制时使用最大并发数？类似 TCP 的滑动窗口


<details>
<summary>手写怎么用 chan 实现定时器</summary>

Doc: https://juejin.cn/post/7113553728954695693

time.NewTicker + for死循环 就ok了

</details>

- 实现一个 hashmap，解决 hash 冲突问题，解决 hash 倾斜问题？
- golang 用 chan 实现排序（快排和归并排序
- 单链表和双链表的反转


<details>
<summary>数组中有N+2个数，其中，N个数出现了偶数次，2个数出现了奇数次（这两个数不相等），请用O(1)的空间复杂度，找出这两个数。</summary>

Doc: https://zhuanlan.zhihu.com/p/373580530

两种方法，hash法或者xor法，推荐hash法。也就是直接把array转hashmap，然后

</details>

- 链表相加 [leetcode 链表相加 - 知乎](https://zhuanlan.zhihu.com/p/480342701)
- 手写“哲学家就餐问题”？
- [使用 chan 下载远程文件，在控制台打印进度条](https://gist.github.com/hxhac/b81e4a21a399567c7d8428709b168349)
- [怎么实现单文件分块并发下载？](https://go.dev/play/p/sVTuMqf6ZHv)


<details>
<summary>翻转slice。能否用golang generics实现一个兼容string和int的SliceReverse的函数？</summary>

URL: https://gist.github.com/hxhac/f5f701cc3d2ae56eda905ce66bb3c7af

方法很多，slices.Reverse()可以直接翻转。直接用fori双迭代器 + 直接交换，是最通用的实现。

</details>

- [实现阻塞读的并发安全Map](https://gist.github.com/hxhac/0edf5f9f056d8199092876ca1bef8ead)
- 高并发下的锁与map读写问题
- 为 sync.WaitGroup 中Wait函数支持 WaitTimeout 功能.
- 手撕代码生产者消费者模型
- 写个递归实现无限级分类
- 写一个验证标准 ipv4 的算法
- [LB](https://go.dev/play/p/7I7Ya3281fW)
- [反向层序遍历二叉树](https://go.dev/play/p/uNepsMLbeqp)
- [反转链表](https://go.dev/play/p/1dhO_6uGasT)
- [???](https://go.dev/play/p/hlzzGprVuxn)
- [convert to base7](https://gist.github.com/hxhac/98339e52845bd88c367e0f4055893dda)
- [gossip](https://gist.github.com/hxhac/164eec10b3fa3400f91e9cd44ec6a9bf)
- [snowflake](https://gist.github.com/hxhac/d20944641ffcfe7ee776b21636a8b415)
- [在线聊天服务](https://gist.github.com/hxhac/ffbdc0100cb5fcf985427302e7d3321f)
- [url使用url.URL定义，而不是string，方便操作](https://go.dev/play/p/CTbEqYxRSoK)
- [LRU](https://go.dev/play/p/G5HXveY6I_v)
- [RingBuffer实现](https://go.dev/play/p/uHZDuVYlET0)
### Search Algo 
- @binary search algo
- [bs, bsc (binary-search)](https://gist.github.com/hxhac/3ca15e0f80245836b8b2b3e8ee44196c)
- *Compare these common search-algo?*
- 分别实现二分查找和哈希查找。用二分查找实现一个数在数组里出现的次数？
- 二分查找，一个数在数组里出现的次数？
