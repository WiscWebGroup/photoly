-- phpMyAdmin SQL Dump
-- version 5.0.4
-- https://www.phpmyadmin.net/
--
-- 主机： localhost
-- 生成日期： 2023-07-10 12:02:28
-- 服务器版本： 5.7.37-log
-- PHP 版本： 7.4.27

SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
START TRANSACTION;
SET time_zone = "+00:00";


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;

--
-- 数据库： `picbed`
--

-- --------------------------------------------------------

--
-- 表的结构 `cred`
--

CREATE TABLE `cred` (
  `cred_id` int(11) NOT NULL,
  `user_id` int(11) DEFAULT NULL,
  `token` varchar(255) DEFAULT NULL,
  `authorization` varchar(255) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

-- --------------------------------------------------------

--
-- 表的结构 `gallery`
--

CREATE TABLE `gallery` (
  `ga_id` int(11) NOT NULL,
  `ga_name` varchar(255) DEFAULT NULL,
  `user_id` int(11) DEFAULT NULL,
  `create_date` datetime DEFAULT NULL,
  `cover_id` int(11) DEFAULT NULL,
  `cover_color` varchar(255) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

-- --------------------------------------------------------

--
-- 表的结构 `gallery_photo`
--

CREATE TABLE `gallery_photo` (
  `gp_id` int(11) NOT NULL,
  `ga_id` int(11) DEFAULT NULL,
  `photo_id` int(11) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

-- --------------------------------------------------------

--
-- 表的结构 `namespace`
--

CREATE TABLE `namespace` (
  `ns_id` int(11) NOT NULL,
  `ns_name` varchar(255) DEFAULT NULL,
  `ns_parent_id` int(11) DEFAULT NULL,
  `user_id` int(11) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

-- --------------------------------------------------------

--
-- 表的结构 `namespace_photo`
--

CREATE TABLE `namespace_photo` (
  `np_id` int(11) NOT NULL,
  `ns_id` int(11) DEFAULT NULL,
  `photo_id` int(11) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

-- --------------------------------------------------------

--
-- 表的结构 `photo`
--

CREATE TABLE `photo` (
  `photo_id` int(11) NOT NULL,
  `photo_name` varchar(255) DEFAULT NULL,
  `upload_date` datetime DEFAULT NULL,
  `format` varchar(255) DEFAULT NULL,
  `ns_id` int(11) DEFAULT NULL,
  `visibility` int(11) DEFAULT NULL,
  `token` varchar(255) DEFAULT NULL,
  `photo_uuid` varchar(255) DEFAULT NULL,
  `user_id` int(11) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

-- --------------------------------------------------------

--
-- 表的结构 `setting`
--

CREATE TABLE `setting` (
  `setting_id` int(11) NOT NULL,
  `setting_name` varchar(255) DEFAULT NULL,
  `setting_value` varchar(255) DEFAULT NULL,
  `note` varchar(255) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- --------------------------------------------------------

--
-- 表的结构 `tag`
--

CREATE TABLE `tag` (
  `tag_id` int(11) NOT NULL,
  `user_id` int(11) NOT NULL,
  `tag_name` varchar(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

-- --------------------------------------------------------

--
-- 表的结构 `tag_photo`
--

CREATE TABLE `tag_photo` (
  `tp_id` int(11) NOT NULL,
  `user_id` int(11) DEFAULT NULL,
  `tag_id` int(11) DEFAULT NULL,
  `photo_id` int(11) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

-- --------------------------------------------------------

--
-- 表的结构 `user`
--

CREATE TABLE `user` (
  `user_id` int(11) NOT NULL,
  `user_name` varchar(255) DEFAULT NULL,
  `email` varchar(255) DEFAULT NULL,
  `create_date` datetime DEFAULT NULL,
  `role` varchar(255) DEFAULT NULL,
  `password` varchar(255) DEFAULT NULL,
  `uuid` varchar(255) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=DYNAMIC;

--
-- 转储表的索引
--

--
-- 表的索引 `cred`
--
ALTER TABLE `cred`
  ADD PRIMARY KEY (`cred_id`) USING BTREE;

--
-- 表的索引 `gallery`
--
ALTER TABLE `gallery`
  ADD PRIMARY KEY (`ga_id`) USING BTREE;

--
-- 表的索引 `gallery_photo`
--
ALTER TABLE `gallery_photo`
  ADD PRIMARY KEY (`gp_id`) USING BTREE,
  ADD KEY `c2` (`photo_id`) USING BTREE,
  ADD KEY `c1` (`ga_id`) USING BTREE;

--
-- 表的索引 `namespace`
--
ALTER TABLE `namespace`
  ADD PRIMARY KEY (`ns_id`) USING BTREE;

--
-- 表的索引 `namespace_photo`
--
ALTER TABLE `namespace_photo`
  ADD PRIMARY KEY (`np_id`) USING BTREE,
  ADD KEY `f3` (`ns_id`) USING BTREE,
  ADD KEY `f4` (`photo_id`) USING BTREE;

--
-- 表的索引 `photo`
--
ALTER TABLE `photo`
  ADD PRIMARY KEY (`photo_id`) USING BTREE;

--
-- 表的索引 `setting`
--
ALTER TABLE `setting`
  ADD PRIMARY KEY (`setting_id`);

--
-- 表的索引 `tag`
--
ALTER TABLE `tag`
  ADD PRIMARY KEY (`tag_id`) USING BTREE;

--
-- 表的索引 `tag_photo`
--
ALTER TABLE `tag_photo`
  ADD PRIMARY KEY (`tp_id`) USING BTREE,
  ADD KEY `f1` (`tag_id`) USING BTREE,
  ADD KEY `f2` (`photo_id`) USING BTREE;

--
-- 表的索引 `user`
--
ALTER TABLE `user`
  ADD PRIMARY KEY (`user_id`) USING BTREE;

--
-- 在导出的表使用AUTO_INCREMENT
--

--
-- 使用表AUTO_INCREMENT `cred`
--
ALTER TABLE `cred`
  MODIFY `cred_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `gallery`
--
ALTER TABLE `gallery`
  MODIFY `ga_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `gallery_photo`
--
ALTER TABLE `gallery_photo`
  MODIFY `gp_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `namespace`
--
ALTER TABLE `namespace`
  MODIFY `ns_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `namespace_photo`
--
ALTER TABLE `namespace_photo`
  MODIFY `np_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `photo`
--
ALTER TABLE `photo`
  MODIFY `photo_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `setting`
--
ALTER TABLE `setting`
  MODIFY `setting_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `tag`
--
ALTER TABLE `tag`
  MODIFY `tag_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `tag_photo`
--
ALTER TABLE `tag_photo`
  MODIFY `tp_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 使用表AUTO_INCREMENT `user`
--
ALTER TABLE `user`
  MODIFY `user_id` int(11) NOT NULL AUTO_INCREMENT;

--
-- 限制导出的表
--

--
-- 限制表 `gallery_photo`
--
ALTER TABLE `gallery_photo`
  ADD CONSTRAINT `c1` FOREIGN KEY (`ga_id`) REFERENCES `gallery` (`ga_id`) ON DELETE CASCADE,
  ADD CONSTRAINT `c2` FOREIGN KEY (`photo_id`) REFERENCES `photo` (`photo_id`) ON DELETE CASCADE;

--
-- 限制表 `namespace_photo`
--
ALTER TABLE `namespace_photo`
  ADD CONSTRAINT `f3` FOREIGN KEY (`ns_id`) REFERENCES `namespace` (`ns_id`) ON DELETE CASCADE,
  ADD CONSTRAINT `f4` FOREIGN KEY (`photo_id`) REFERENCES `photo` (`photo_id`) ON DELETE CASCADE;

--
-- 限制表 `tag_photo`
--
ALTER TABLE `tag_photo`
  ADD CONSTRAINT `f1` FOREIGN KEY (`tag_id`) REFERENCES `tag` (`tag_id`) ON DELETE CASCADE,
  ADD CONSTRAINT `f2` FOREIGN KEY (`photo_id`) REFERENCES `photo` (`photo_id`) ON DELETE CASCADE;
COMMIT;

/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;

INSERT INTO setting (setting_name, setting_value, note) values ('ping', 'pong', 'use for testing connection');
INSERT INTO setting (setting_name, setting_value, note) values ('SignUp', '1', 'use for determine if new user could sign up');
INSERT INTO setting (setting_name, setting_value, note) values ('SSafeUUID', '1', 'use for determine if SSafe is enabled');
INSERT INTO setting (setting_name, setting_value, note) values ('TokenDuration', '30', 'use for determine the duration of token');

INSERT INTO user (user_name, email, role, password, uuid) values ('admin', 'admin@a.com', 'admin', 'Z2pWGE3xacr3Vbig86OriQ==', '2b26e85f-ffb0-4c5a-a70f-4782ce782cd6');

