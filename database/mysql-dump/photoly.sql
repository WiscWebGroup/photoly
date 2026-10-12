-- Photoly Flask fresh-deployment schema. Authoritative bootstrap: flask --app app:create_app init-db.
-- No DROP statements, passwords, or legacy encrypted administrator seed.
SET NAMES utf8mb4;



CREATE TABLE IF NOT EXISTS setting (
	setting_id INTEGER NOT NULL AUTO_INCREMENT,
	setting_name VARCHAR(255),
	setting_value VARCHAR(255),
	note VARCHAR(255),
	CONSTRAINT pk_setting PRIMARY KEY (setting_id),
	CONSTRAINT uq_setting_setting_name UNIQUE (setting_name)
)

;


CREATE TABLE IF NOT EXISTS user (
	user_id INTEGER NOT NULL AUTO_INCREMENT,
	user_name VARCHAR(255),
	email VARCHAR(255),
	create_date DATETIME,
	`role` VARCHAR(255),
	password VARCHAR(255),
	uuid VARCHAR(255),
	CONSTRAINT pk_user PRIMARY KEY (user_id),
	CONSTRAINT uq_user_email UNIQUE (email),
	CONSTRAINT uq_user_uuid UNIQUE (uuid)
)

;


CREATE TABLE IF NOT EXISTS cred (
	cred_id INTEGER NOT NULL AUTO_INCREMENT,
	user_id INTEGER,
	token VARCHAR(255),
	authorization VARCHAR(255),
	CONSTRAINT pk_cred PRIMARY KEY (cred_id),
	CONSTRAINT fk_cred_user_id_user FOREIGN KEY(user_id) REFERENCES user (user_id) ON DELETE CASCADE,
	CONSTRAINT uq_cred_token UNIQUE (token)
)

;

CREATE INDEX ix_cred_user_id ON cred (user_id);


CREATE TABLE IF NOT EXISTS gallery (
	ga_id INTEGER NOT NULL AUTO_INCREMENT,
	ga_name VARCHAR(255),
	user_id INTEGER,
	create_date DATETIME,
	cover_id INTEGER,
	cover_color VARCHAR(255),
	CONSTRAINT pk_gallery PRIMARY KEY (ga_id),
	CONSTRAINT fk_gallery_user_id_user FOREIGN KEY(user_id) REFERENCES user (user_id) ON DELETE CASCADE
)

;

CREATE INDEX ix_gallery_user_id ON gallery (user_id);


CREATE TABLE IF NOT EXISTS namespace (
	ns_id INTEGER NOT NULL AUTO_INCREMENT,
	ns_name VARCHAR(255),
	ns_parent_id INTEGER,
	user_id INTEGER,
	CONSTRAINT pk_namespace PRIMARY KEY (ns_id),
	CONSTRAINT fk_namespace_user_id_user FOREIGN KEY(user_id) REFERENCES user (user_id) ON DELETE CASCADE
)

;

CREATE INDEX ix_namespace_ns_parent_id ON namespace (ns_parent_id);

CREATE INDEX ix_namespace_user_id ON namespace (user_id);


CREATE TABLE IF NOT EXISTS tag (
	tag_id INTEGER NOT NULL AUTO_INCREMENT,
	user_id INTEGER NOT NULL,
	tag_name VARCHAR(255) NOT NULL,
	CONSTRAINT pk_tag PRIMARY KEY (tag_id),
	CONSTRAINT fk_tag_user_id_user FOREIGN KEY(user_id) REFERENCES user (user_id) ON DELETE CASCADE
)

;

CREATE INDEX ix_tag_user_id ON tag (user_id);


CREATE TABLE IF NOT EXISTS photo (
	photo_id INTEGER NOT NULL AUTO_INCREMENT,
	photo_name VARCHAR(255),
	upload_date DATETIME,
	format VARCHAR(255),
	ns_id INTEGER,
	visibility INTEGER,
	token VARCHAR(255),
	photo_uuid VARCHAR(255),
	user_id INTEGER,
	CONSTRAINT pk_photo PRIMARY KEY (photo_id),
	CONSTRAINT fk_photo_ns_id_namespace FOREIGN KEY(ns_id) REFERENCES namespace (ns_id),
	CONSTRAINT uq_photo_token UNIQUE (token),
	CONSTRAINT uq_photo_photo_uuid UNIQUE (photo_uuid),
	CONSTRAINT fk_photo_user_id_user FOREIGN KEY(user_id) REFERENCES user (user_id) ON DELETE CASCADE
)

;

CREATE INDEX ix_photo_ns_id ON photo (ns_id);

CREATE INDEX ix_photo_user_id ON photo (user_id);


CREATE TABLE IF NOT EXISTS gallery_photo (
	gp_id INTEGER NOT NULL AUTO_INCREMENT,
	ga_id INTEGER,
	photo_id INTEGER,
	CONSTRAINT pk_gallery_photo PRIMARY KEY (gp_id),
	CONSTRAINT uq_gallery_photo_ga_id UNIQUE (ga_id, photo_id),
	CONSTRAINT fk_gallery_photo_ga_id_gallery FOREIGN KEY(ga_id) REFERENCES gallery (ga_id) ON DELETE CASCADE,
	CONSTRAINT fk_gallery_photo_photo_id_photo FOREIGN KEY(photo_id) REFERENCES photo (photo_id) ON DELETE CASCADE
)

;

CREATE INDEX ix_gallery_photo_ga_id ON gallery_photo (ga_id);

CREATE INDEX ix_gallery_photo_photo_id ON gallery_photo (photo_id);


CREATE TABLE IF NOT EXISTS namespace_photo (
	np_id INTEGER NOT NULL AUTO_INCREMENT,
	ns_id INTEGER,
	photo_id INTEGER,
	CONSTRAINT pk_namespace_photo PRIMARY KEY (np_id),
	CONSTRAINT fk_namespace_photo_ns_id_namespace FOREIGN KEY(ns_id) REFERENCES namespace (ns_id) ON DELETE CASCADE,
	CONSTRAINT fk_namespace_photo_photo_id_photo FOREIGN KEY(photo_id) REFERENCES photo (photo_id) ON DELETE CASCADE
)

;

CREATE INDEX ix_namespace_photo_ns_id ON namespace_photo (ns_id);

CREATE INDEX ix_namespace_photo_photo_id ON namespace_photo (photo_id);


CREATE TABLE IF NOT EXISTS photo_extra (
	extra_photo_id INTEGER NOT NULL AUTO_INCREMENT,
	f_photo_id INTEGER NOT NULL,
	extra_photo_gps_lat DOUBLE,
	extra_photo_gps_lon DOUBLE,
	extra_photo_camera VARCHAR(255),
	extra_photo_lens VARCHAR(255),
	extra_photo_aperture DOUBLE,
	extra_photo_focal INTEGER,
	extra_photo_iso INTEGER,
	extra_photo_shutter_sec INTEGER,
	extra_photo_taken_date DATETIME,
	extra_photo_note VARCHAR(255),
	CONSTRAINT pk_photo_extra PRIMARY KEY (extra_photo_id),
	CONSTRAINT uq_photo_extra_f_photo_id UNIQUE (f_photo_id),
	CONSTRAINT fk_photo_extra_f_photo_id_photo FOREIGN KEY(f_photo_id) REFERENCES photo (photo_id) ON DELETE CASCADE
)

;


CREATE TABLE IF NOT EXISTS tag_photo (
	tp_id INTEGER NOT NULL AUTO_INCREMENT,
	user_id INTEGER,
	tag_id INTEGER,
	photo_id INTEGER,
	CONSTRAINT pk_tag_photo PRIMARY KEY (tp_id),
	CONSTRAINT uq_tag_photo_tag_id UNIQUE (tag_id, photo_id),
	CONSTRAINT fk_tag_photo_user_id_user FOREIGN KEY(user_id) REFERENCES user (user_id) ON DELETE CASCADE,
	CONSTRAINT fk_tag_photo_tag_id_tag FOREIGN KEY(tag_id) REFERENCES tag (tag_id) ON DELETE CASCADE,
	CONSTRAINT fk_tag_photo_photo_id_photo FOREIGN KEY(photo_id) REFERENCES photo (photo_id) ON DELETE CASCADE
)

;

CREATE INDEX ix_tag_photo_photo_id ON tag_photo (photo_id);

CREATE INDEX ix_tag_photo_tag_id ON tag_photo (tag_id);

CREATE INDEX ix_tag_photo_user_id ON tag_photo (user_id);

INSERT IGNORE INTO setting (setting_name, setting_value, note) VALUES ('ping', 'pong', 'use for testing connection');

INSERT IGNORE INTO setting (setting_name, setting_value, note) VALUES ('SignUp', '1', 'use for determine if new user could sign up');

INSERT IGNORE INTO setting (setting_name, setting_value, note) VALUES ('SSafeUUID', '1', 'use for determine if SSafe is enabled');

INSERT IGNORE INTO setting (setting_name, setting_value, note) VALUES ('TokenDuration', '30', 'use for determine the duration of token');
