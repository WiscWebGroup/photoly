package org.chengbing.service.impl;

import org.chengbing.entity.Photo;
import org.chengbing.entity.TagPhoto;
import org.chengbing.dao.TagPhotoMapper;
import org.chengbing.service.ITagPhotoService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.List;

/**
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Service
public class TagPhotoServiceImpl extends ServiceImpl<TagPhotoMapper, TagPhoto> implements ITagPhotoService {
    @Resource
    TagPhotoMapper tagPhotoMapper;
    @Override
    public List<Photo> queryPhotoByTagName(Integer userId, String tagName) {
        return tagPhotoMapper.queryPhotoByTagName(userId, tagName);
    }
}
