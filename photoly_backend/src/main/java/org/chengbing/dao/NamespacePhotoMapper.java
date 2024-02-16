package org.chengbing.dao;

import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.chengbing.entity.NamespacePhoto;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import org.chengbing.entity.Photo;

import java.util.List;

/**
 * <p>
 *  Mapper 接口
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Mapper
public interface NamespacePhotoMapper extends BaseMapper<NamespacePhoto> {
    List<Photo> queryPhotoByNsName(@Param("userId") Integer userId, @Param("nsName") String nsName);
}
