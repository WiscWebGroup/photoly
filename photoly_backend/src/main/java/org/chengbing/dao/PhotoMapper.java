package org.chengbing.dao;

import org.apache.ibatis.annotations.Mapper;
import org.chengbing.entity.Photo;
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
public interface PhotoMapper extends BaseMapper<Photo> {

}
