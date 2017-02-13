CREATE TABLE `folder_synckey` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_uid` bigint(20) NOT NULL,
  `device_id` varchar(64) NOT NULL,
  `history_id` bigint(20) NOT NULL,
  `timestamp` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY (`user_uid`, `device_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `virtual_folder` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_uid` bigint(20) NOT NULL,
  `device_id` varchar(64) NOT NULL,
  `folder_id` bigint(20) NOT NULL,
  `parent_folder_id` bigint(20) NOT NULL,
  `name` varchar(256) NOT NULL,
  `last_history_id` bigint(20) NOT NULL,
  `timestamp` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY (`user_uid`, `device_id`, `folder_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `email_synckey` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_uid` bigint(20) NOT NULL,
  `device_id` varchar(64) NOT NULL,
  `folder_id` bigint(20) NOT NULL,
  `history_id` bigint(20) NOT NULL,
  `timestamp` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY (`user_uid`, `device_id`, `folder_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

CREATE TABLE `virtual_email` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_uid` bigint(20) NOT NULL,
  `device_id` varchar(64) NOT NULL,
  `folder_id` bigint(20) NOT NULL,
  `email_id` bigint(20) NOT NULL,
  `seen` bool NOT NULL,
  `timestamp` datetime NOT NULL,
  `last_history_id` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY (`user_uid`, `device_id`, `folder_id`, `email_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
