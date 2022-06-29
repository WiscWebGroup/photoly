package org.chengbing.service.impl;

import org.chengbing.entity.Namespace;
import org.chengbing.dao.NamespaceMapper;
import org.chengbing.service.INamespaceService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;

***REMOVED***
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@Service
public class NamespaceServiceImpl extends ServiceImpl<NamespaceMapper, Namespace> implements INamespaceService {

    @Resource
    NamespaceMapper mapper;

    @Override
    public Integer insertNamespace(String name, Integer parentId, Integer userId) {
        Namespace namespace = new Namespace();
        namespace.setNsName(name);
        namespace.setNsParentId(parentId);
        namespace.setUserId(userId);
        return mapper.insert(namespace);
***REMOVED***
***REMOVED***
