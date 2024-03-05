package org.chengbing.service.impl;

import org.chengbing.entity.PhotoExtra;
import org.chengbing.dao.PhotoExtraMapper;
import org.chengbing.service.IPhotoExtraService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.LinkedHashMap;
import java.util.List;

/**
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2024-03-04
 */
@Service
public class PhotoExtraServiceImpl extends ServiceImpl<PhotoExtraMapper, PhotoExtra> implements IPhotoExtraService {
    @Resource
    PhotoExtraMapper mapper;
    @Override
    public List<LinkedHashMap<String, Object>> queryGlobalPhoto(Integer userId) {
        return mapper.queryGlobalPhoto(userId);
    }

    @Override
    public List<LinkedHashMap<String, Object>> queryGalleryPhoto(Integer userId, Integer gaId) {
        return mapper.queryGalleryPhoto(userId, gaId);
    }
}
