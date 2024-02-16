package org.chengbing.service.impl;

import org.chengbing.entity.NamespacePhoto;
import org.chengbing.dao.NamespacePhotoMapper;
import org.chengbing.entity.Photo;
import org.chengbing.service.INamespacePhotoService;
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
public class NamespacePhotoServiceImpl extends ServiceImpl<NamespacePhotoMapper, NamespacePhoto> implements INamespacePhotoService {
    @Resource
    NamespacePhotoMapper namespacePhotoMapper;
    @Override
    public List<Photo> queryPhotoByNsName(Integer userId, String nsName) {
        return namespacePhotoMapper.queryPhotoByNsName(userId, nsName);
    }
}
