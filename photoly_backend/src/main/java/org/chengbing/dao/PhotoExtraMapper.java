package org.chengbing.dao;

import org.apache.ibatis.annotations.Mapper;
import org.chengbing.entity.PhotoExtra;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;

import java.util.LinkedHashMap;
import java.util.List;

/**
 * <p>
 *  Mapper 接口
 * </p>
 *
 * @author HaroldCI
 * @since 2024-03-04
 */
@Mapper
public interface PhotoExtraMapper extends BaseMapper<PhotoExtra> {
    List<LinkedHashMap<String, Object>> queryGlobalPhoto(Integer userId);

    List<LinkedHashMap<String, Object>> queryGalleryPhoto(Integer userId, Integer gaId);
}
