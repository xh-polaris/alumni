/**
 * 本地联调用的演示数据（仅开发环境使用，不属于部署产物）。
 *
 * 用法：
 *   docker exec -i alumni-mongo mongosh alumni --quiet < dev/seed-demo.js
 *
 * 设计意图：
 * - 四个分会的联络人 / 微信 / 说明，用于注册结果弹层与「分会管理」页；
 * - 名册里放一条与「开发测试用户」完全一致的记录，这样用户端补全资料时
 *   可以真的走通「自动认证」路径；填别的值则会停留在待认证；
 * - 覆盖 pending / alumni / guest 三种身份与 chapter_admin 权限，
 *   让管理端的筛选、认证操作与「管理员配置」都有数据。
 *
 * 注意：本脚本只在集合为空时插入（幂等），可重复执行。
 */

/* eslint-disable */
const now = new Date();
const day = 24 * 60 * 60 * 1000;
const unix = (offsetMs) => Math.floor((now.getTime() + offsetMs) / 1000);

const chapterByCode = {};
db.chapter.find({}).forEach((item) => {
  chapterByCode[item.code] = item._id.toString();
});

const need = ["ningbo", "shanghai", "hangzhou", "beijing"];
need.forEach((code) => {
  if (!chapterByCode[code]) {
    throw new Error("缺少分会 " + code + "，请先启动一次服务端以初始化四个分会");
  }
});

// ---------------------------------------------------------------- 分会联络人
// 注意：这里写的是**库里的字段名**（snake_case），不是接口 JSON 名。
// 早期误写成 qrCodeURL，后端按 qr_code_url 读取，于是写的二维码读不出来。
const contacts = {
  ningbo: {
    name: "王老师",
    wechat: "ningbo_alumni",
    phone: "13900000001",
    description: "负责宁波分会日常联络、返校活动与名册核验",
    qr_code_url: "",
  },
  shanghai: {
    name: "李老师",
    wechat: "sh_alumni",
    phone: "13900000002",
    description: "负责上海分会日常联络、活动组织与名册核验",
    qr_code_url: "",
  },
  hangzhou: {
    name: "张老师",
    wechat: "hz_alumni",
    phone: "13900000003",
    description: "负责杭州分会日常联络与活动组织",
    qr_code_url: "",
  },
  beijing: {
    name: "陈老师",
    wechat: "bj_alumni",
    phone: "13900000004",
    description: "负责北京分会日常联络与活动组织",
    qr_code_url: "",
  },
};

Object.entries(contacts).forEach(([code, contact]) => {
  db.chapter.updateOne({ _id: new ObjectId(chapterByCode[code]) }, { $set: { contact } });
});

// --------------------------------------------------------------------- 人员
const shanghai = chapterByCode.shanghai;
const ningbo = chapterByCode.ningbo;
const hangzhou = chapterByCode.hangzhou;

const users = [
  {
    // 与后端 DevMockUserID 一致：用户端「?env=dev」登录后就是这个人。
    // 故意留成待认证 + 缺毕业年份/出生日期，用来演示补全资料 -> 自动认证。
    _id: new ObjectId("66a000000000000000000001"),
    avatar: "",
    name: "开发测试用户",
    gender: NumberLong(1),
    phone: "13800000000",
    wx_id: "",
    hometown: "宁波北仑",
    chapter_id: shanghai,
    graduation_year: NumberLong(0),
    birth_date: "",
    member_role: "pending",
    admin_role: "none",
    admin_chapter_id: "",
    verification_method: "none",
    educations: [],
    employments: [],
    role: "pending",
    status: NumberLong(0),
    create_time: now,
    update_time: now,
  },
  {
    _id: new ObjectId(),
    avatar: "",
    name: "陈默",
    gender: NumberLong(1),
    phone: "13800000002",
    wx_id: "chenmo",
    hometown: "宁波北仑",
    chapter_id: shanghai,
    graduation_year: NumberLong(2018),
    birth_date: "2000-05-20",
    member_role: "alumni",
    admin_role: "none",
    admin_chapter_id: "",
    verification_method: "roster",
    verified_at: now,
    educations: [
      {
        phase: "高中",
        school: "宁波市北仑中学",
        year: NumberLong(2015),
        province_code: "330000",
        province_name: "浙江省",
        city_code: "330200",
        city_name: "宁波市",
      },
      {
        phase: "本科",
        school: "同济大学",
        year: NumberLong(2018),
        province_code: "310000",
        province_name: "上海市",
        city_code: "310104",
        city_name: "徐汇区",
      },
    ],
    employments: [
      {
        organization: "澄明设计",
        position: "产品设计师",
        industry: "科学研究和技术服务业",
        entry: NumberLong(2021),
        departure: NumberLong(0),
        province_code: "310000",
        province_name: "上海市",
        city_code: "310104",
        city_name: "徐汇区",
      },
    ],
    role: "alumni",
    status: NumberLong(0),
    create_time: now,
    update_time: now,
  },
  {
    _id: new ObjectId(),
    avatar: "",
    name: "林然",
    gender: NumberLong(2),
    phone: "13800000003",
    wx_id: "",
    hometown: "宁波北仑",
    chapter_id: ningbo,
    graduation_year: NumberLong(2016),
    birth_date: "1998-03-08",
    member_role: "pending",
    admin_role: "none",
    admin_chapter_id: "",
    verification_method: "none",
    educations: [],
    employments: [],
    role: "pending",
    status: NumberLong(0),
    create_time: now,
    update_time: now,
  },
  {
    _id: new ObjectId(),
    avatar: "",
    name: "赵嘉",
    gender: NumberLong(1),
    phone: "13800000004",
    wx_id: "",
    hometown: "杭州",
    chapter_id: hangzhou,
    graduation_year: NumberLong(2020),
    birth_date: "2002-11-30",
    member_role: "guest",
    admin_role: "none",
    admin_chapter_id: "",
    verification_method: "manual",
    verified_at: now,
    educations: [],
    employments: [],
    role: "guest",
    status: NumberLong(0),
    create_time: now,
    update_time: now,
  },
  {
    // 分会管理员：admin_chapter_id 固定为上海分会，不随个人分会切换而变。
    _id: new ObjectId(),
    avatar: "",
    name: "周宁",
    gender: NumberLong(1),
    phone: "13800000005",
    wx_id: "zhouning",
    hometown: "宁波北仑",
    chapter_id: shanghai,
    graduation_year: NumberLong(2012),
    birth_date: "1994-07-16",
    member_role: "alumni",
    admin_role: "chapter_admin",
    admin_chapter_id: shanghai,
    verification_method: "manual",
    verified_at: now,
    educations: [],
    employments: [],
    role: "alumni",
    status: NumberLong(0),
    create_time: now,
    update_time: now,
  },
];

let usersCreated = 0;
users.forEach((item) => {
  if (db.user.countDocuments({ _id: item._id }) === 0) {
    db.user.insertOne(item);
    usersCreated++;
  }
});

// --------------------------------------------------------------------- 名册
const roster = [
  { name: "开发测试用户", graduation_year: NumberLong(2018), birth_date: "2000-05-20" },
  { name: "陈默", graduation_year: NumberLong(2018), birth_date: "2000-05-20" },
  { name: "林然", graduation_year: NumberLong(2016), birth_date: "1998-03-08" },
  { name: "赵嘉", graduation_year: NumberLong(2020), birth_date: "2002-11-30" },
];

let rosterCreated = 0;
roster.forEach((item) => {
  const filter = {
    normalized_name: item.name,
    graduation_year: item.graduation_year,
    birth_date: item.birth_date,
  };
  if (db.alumni_roster.countDocuments(filter) === 0) {
    db.alumni_roster.insertOne(
      Object.assign({}, filter, { name: item.name, create_time: now, update_time: now }),
    );
    rosterCreated++;
  }
});

// --------------------------------------------------------------------- 资讯
const articles = [
  {
    title: "沪甬两地校友企业交流活动举行",
    summary: "三十余位校友走进张江，围绕智能制造与品牌出海展开交流。",
    cover: "",
    wechat_url: "https://mp.weixin.qq.com/s/alumni-shanghai-001",
    source: "上海分会",
    author: "上海分会秘书处",
    chapter_id: shanghai,
    publish_time: new Date(now.getTime() - 2 * day),
    sort_order: NumberLong(10),
    publish_status: "published",
    deleted: false,
    create_time: new Date(now.getTime() - 2 * day),
    update_time: now,
  },
  {
    title: "宁波分会校友返校日活动预告",
    summary: "回到钟楼下的操场，看看翻新后的体育馆，与老师同学再见一面。",
    cover: "",
    wechat_url: "https://mp.weixin.qq.com/s/alumni-ningbo-002",
    source: "宁波分会",
    author: "宁波分会秘书处",
    chapter_id: ningbo,
    publish_time: new Date(now.getTime() - 4 * day),
    sort_order: NumberLong(9),
    publish_status: "published",
    deleted: false,
    create_time: new Date(now.getTime() - 4 * day),
    update_time: now,
  },
  {
    title: "杭州分会春季徒步活动回顾",
    summary: "从九溪到龙井，二十公里山路走完，也聊完了各自的近况。",
    cover: "",
    wechat_url: "https://mp.weixin.qq.com/s/alumni-hangzhou-003",
    source: "杭州分会",
    author: "杭州分会秘书处",
    chapter_id: hangzhou,
    publish_time: new Date(now.getTime() - 7 * day),
    sort_order: NumberLong(8),
    publish_status: "published",
    deleted: false,
    create_time: new Date(now.getTime() - 7 * day),
    update_time: now,
  },
];

let articlesCreated = 0;
articles.forEach((item) => {
  if (db.article.countDocuments({ title: item.title, chapter_id: item.chapter_id }) === 0) {
    db.article.insertOne(item);
    articlesCreated++;
  }
});

// --------------------------------------------------------------------- 活动
const activities = [
  {
    cover: "",
    name: "北仑青年校友夏日交流会",
    location: "上海",
    exact_location: '{"name":"杨浦区创智天地","address":"上海市杨浦区淞沪路","latitude":31.3063,"longitude":121.5138}',
    sponsor: "上海分会",
    chapter_id: shanghai,
    start: NumberLong(unix(40 * day)),
    register_start: new Date(now.getTime() - 1 * day),
    register_end: new Date(now.getTime() + 30 * day),
    description: "面向在沪工作的青年校友，围绕行业交流、职业发展与生活分享展开。",
    contact: "李老师 13900000002",
    limit: NumberLong(50),
    status: NumberLong(0),
    create_time: now,
    update_time: now,
  },
  {
    cover: "",
    name: "宁波分会校友返校日",
    location: "宁波",
    exact_location: '{"name":"宁波市北仑中学","address":"浙江省宁波市北仑区","latitude":29.9008,"longitude":121.8442}',
    sponsor: "宁波分会",
    chapter_id: ningbo,
    start: NumberLong(unix(25 * day)),
    register_start: new Date(now.getTime() - 3 * day),
    register_end: new Date(now.getTime() + 20 * day),
    description: "参观翻新后的教学楼与体育馆，与老教师座谈，晚上安排校友聚餐。",
    contact: "王老师 13900000001",
    limit: NumberLong(-1),
    status: NumberLong(0),
    create_time: now,
    update_time: now,
  },
  {
    cover: "",
    name: "杭州分会春季徒步（已截止）",
    location: "杭州",
    exact_location: '{"name":"九溪入口","address":"浙江省杭州市西湖区"}',
    sponsor: "杭州分会",
    chapter_id: hangzhou,
    start: NumberLong(unix(-10 * day)),
    register_start: new Date(now.getTime() - 30 * day),
    register_end: new Date(now.getTime() - 20 * day),
    description: "九溪到龙井的经典路线，用来演示「报名已截止」状态。",
    contact: "张老师 13900000003",
    limit: NumberLong(20),
    status: NumberLong(0),
    create_time: now,
    update_time: now,
  },
];

let activitiesCreated = 0;
activities.forEach((item) => {
  if (db.activity.countDocuments({ name: item.name, chapter_id: item.chapter_id }) === 0) {
    db.activity.insertOne(item);
    activitiesCreated++;
  }
});

// --------------------------------------------------------------------- 报名
const firstActivity = db.activity.findOne({ name: "北仑青年校友夏日交流会" });
let registrationsCreated = 0;
if (firstActivity) {
  const activityId = firstActivity._id.toString();
  const registrations = [
    { name: "陈默", phone: "13800000002", check_in: true },
    { name: "赵嘉", phone: "13800000004", check_in: false },
  ];
  registrations.forEach((item) => {
    if (db.register.countDocuments({ activity_id: activityId, phone: item.phone }) === 0) {
      db.register.insertOne({
        activity_id: activityId,
        chapter_id: firstActivity.chapter_id,
        user_id: "",
        name: item.name,
        phone: item.phone,
        check_in: item.check_in,
        status: NumberLong(0),
        create_time: now,
        update_time: now,
      });
      registrationsCreated++;
    }
  });
}

print("seed 完成：");
print("  用户新增 " + usersCreated + " 条，名册新增 " + rosterCreated + " 条");
print("  资讯新增 " + articlesCreated + " 条，活动新增 " + activitiesCreated + " 条，报名新增 " + registrationsCreated + " 条");
print("  分会联络人已写入 4 个分会");
