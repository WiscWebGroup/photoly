package org.chengbing.dao;

import org.apache.ibatis.annotations.Mapper;
import org.chengbing.entity.Photo;
import org.chengbing.entity.TagPhoto;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;

import java.util.LinkedHashMap;
import java.util.List;

***REMOVED***
 * <p>
 *  Mapper 接口
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@Mapper
public interface TagPhotoMapper extends BaseMapper<TagPhoto> {
    List<Photo> selectByTags(Integer userId, List list, Integer totalNum);
    List<LinkedHashMap<String, Object>> selectTagByPhoto(Integer photoId);
***REMOVED***
