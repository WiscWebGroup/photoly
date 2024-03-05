/*
 Navicat Premium Data Transfer

 Source Server         : CBR3
 Source Server Type    : MySQL
 Source Server Version : 80032
 Source Host           : localhost:3306
 Source Schema         : photoly

 Target Server Type    : MySQL
 Target Server Version : 80032
 File Encoding         : 65001

 Date: 04/03/2024 21:16:37
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for cred
-- ----------------------------
DROP TABLE IF EXISTS `cred`;
CREATE TABLE `cred`  (
  `cred_id` int(0) NOT NULL AUTO_INCREMENT,
  `user_id` int(0) NULL DEFAULT NULL,
  `token` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `authorization` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  PRIMARY KEY (`cred_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 39 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for gallery
-- ----------------------------
DROP TABLE IF EXISTS `gallery`;
CREATE TABLE `gallery`  (
  `ga_id` int(0) NOT NULL AUTO_INCREMENT,
  `ga_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `user_id` int(0) NULL DEFAULT NULL,
  `create_date` datetime(0) NULL DEFAULT NULL,
  `cover_id` int(0) NULL DEFAULT NULL,
  `cover_color` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  PRIMARY KEY (`ga_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 73 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for gallery_photo
-- ----------------------------
DROP TABLE IF EXISTS `gallery_photo`;
CREATE TABLE `gallery_photo`  (
  `gp_id` int(0) NOT NULL AUTO_INCREMENT,
  `ga_id` int(0) NULL DEFAULT NULL,
  `photo_id` int(0) NULL DEFAULT NULL,
  PRIMARY KEY (`gp_id`) USING BTREE,
  INDEX `c2`(`photo_id`) USING BTREE,
  INDEX `c1`(`ga_id`) USING BTREE,
  CONSTRAINT `c1` FOREIGN KEY (`ga_id`) REFERENCES `gallery` (`ga_id`) ON DELETE CASCADE ON UPDATE RESTRICT,
  CONSTRAINT `c2` FOREIGN KEY (`photo_id`) REFERENCES `photo` (`photo_id`) ON DELETE CASCADE ON UPDATE RESTRICT
) ENGINE = InnoDB AUTO_INCREMENT = 35 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for namespace
-- ----------------------------
DROP TABLE IF EXISTS `namespace`;
CREATE TABLE `namespace`  (
  `ns_id` int(0) NOT NULL AUTO_INCREMENT,
  `ns_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `ns_parent_id` int(0) NULL DEFAULT NULL,
  `user_id` int(0) NULL DEFAULT NULL,
  PRIMARY KEY (`ns_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 75 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for namespace_photo
-- ----------------------------
DROP TABLE IF EXISTS `namespace_photo`;
CREATE TABLE `namespace_photo`  (
  `np_id` int(0) NOT NULL AUTO_INCREMENT,
  `ns_id` int(0) NULL DEFAULT NULL,
  `photo_id` int(0) NULL DEFAULT NULL,
  PRIMARY KEY (`np_id`) USING BTREE,
  INDEX `f3`(`ns_id`) USING BTREE,
  INDEX `f4`(`photo_id`) USING BTREE,
  CONSTRAINT `f3` FOREIGN KEY (`ns_id`) REFERENCES `namespace` (`ns_id`) ON DELETE CASCADE ON UPDATE RESTRICT,
  CONSTRAINT `f4` FOREIGN KEY (`photo_id`) REFERENCES `photo` (`photo_id`) ON DELETE CASCADE ON UPDATE RESTRICT
) ENGINE = InnoDB AUTO_INCREMENT = 1 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for photo
-- ----------------------------
DROP TABLE IF EXISTS `photo`;
CREATE TABLE `photo`  (
  `photo_id` int(0) NOT NULL AUTO_INCREMENT,
  `photo_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `upload_date` datetime(0) NULL DEFAULT NULL,
  `format` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `ns_id` int(0) NULL DEFAULT NULL,
  `visibility` int(0) NULL DEFAULT NULL,
  `token` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `photo_uuid` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `user_id` int(0) NULL DEFAULT NULL,
  PRIMARY KEY (`photo_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 151 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for photo_extra
-- ----------------------------
DROP TABLE IF EXISTS `photo_extra`;
CREATE TABLE `photo_extra`  (
  `extra_photo_id` int(0) NOT NULL AUTO_INCREMENT,
  `f_photo_id` int(0) NOT NULL,
  `extra_photo_gps_lat` double NULL DEFAULT NULL,
  `extra_photo_gps_lon` double NULL DEFAULT NULL,
  `extra_photo_camera` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `extra_photo_lens` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `extra_photo_aperture` double NULL DEFAULT NULL,
  `extra_photo_focal` int(0) NULL DEFAULT NULL,
  `extra_photo_iso` int(0) NULL DEFAULT NULL,
  `extra_photo_shutter_sec` int(0) NULL DEFAULT NULL COMMENT 'this is the int for 1/<num>',
  `extra_photo_taken_date` datetime(0) NULL DEFAULT NULL,
  `extra_photo_note` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL COMMENT 'a simple note for the photo',
  PRIMARY KEY (`extra_photo_id`) USING BTREE,
  INDEX `f_photo_id`(`f_photo_id`) USING BTREE,
  CONSTRAINT `f_photo_id` FOREIGN KEY (`f_photo_id`) REFERENCES `photo` (`photo_id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for setting
-- ----------------------------
DROP TABLE IF EXISTS `setting`;
CREATE TABLE `setting`  (
  `setting_id` int(0) NOT NULL AUTO_INCREMENT,
  `setting_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `setting_value` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `note` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  PRIMARY KEY (`setting_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 5 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for tag
-- ----------------------------
DROP TABLE IF EXISTS `tag`;
CREATE TABLE `tag`  (
  `tag_id` int(0) NOT NULL AUTO_INCREMENT,
  `user_id` int(0) NOT NULL,
  `tag_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  PRIMARY KEY (`tag_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 65 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for tag_photo
-- ----------------------------
DROP TABLE IF EXISTS `tag_photo`;
CREATE TABLE `tag_photo`  (
  `tp_id` int(0) NOT NULL AUTO_INCREMENT,
  `user_id` int(0) NULL DEFAULT NULL,
  `tag_id` int(0) NULL DEFAULT NULL,
  `photo_id` int(0) NULL DEFAULT NULL,
  PRIMARY KEY (`tp_id`) USING BTREE,
  INDEX `f1`(`tag_id`) USING BTREE,
  INDEX `f2`(`photo_id`) USING BTREE,
  CONSTRAINT `f1` FOREIGN KEY (`tag_id`) REFERENCES `tag` (`tag_id`) ON DELETE CASCADE ON UPDATE RESTRICT,
  CONSTRAINT `f2` FOREIGN KEY (`photo_id`) REFERENCES `photo` (`photo_id`) ON DELETE CASCADE ON UPDATE RESTRICT
) ENGINE = InnoDB AUTO_INCREMENT = 30 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Table structure for user
-- ----------------------------
DROP TABLE IF EXISTS `user`;
CREATE TABLE `user`  (
  `user_id` int(0) NOT NULL AUTO_INCREMENT,
  `user_name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `email` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `create_date` datetime(0) NULL DEFAULT NULL,
  `role` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `password` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  `uuid` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NULL DEFAULT NULL,
  PRIMARY KEY (`user_id`) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 42 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci ROW_FORMAT = Dynamic;

SET FOREIGN_KEY_CHECKS = 1;

INSERT INTO setting (setting_name, setting_value, note) values ('ping', 'pong', 'use for testing connection');
INSERT INTO setting (setting_name, setting_value, note) values ('SignUp', '1', 'use for determine if new user could sign up');
INSERT INTO setting (setting_name, setting_value, note) values ('SSafeUUID', '1', 'use for determine if SSafe is enabled');
INSERT INTO setting (setting_name, setting_value, note) values ('TokenDuration', '30', 'use for determine the duration of token');

INSERT INTO user (user_name, email, role, password, uuid) values ('admin', 'admin@a.com', 'admin', 'Z2pWGE3xacr3Vbig86OriQ==', '2b26e85f-ffb0-4c5a-a70f-4782ce782cd6');

