package org.chengbing.dao;

import org.apache.ibatis.annotations.Mapper;
import org.chengbing.entity.User;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;

/**
 * <p>
 *  Mapper 接口
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Mapper
public interface UserMapper extends BaseMapper<User> {

}
