package org.chengbing.service;

import org.chengbing.entity.Namespace;
import com.baomidou.mybatisplus.extension.service.IService;

***REMOVED***
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
public interface INamespaceService extends IService<Namespace> {
    Integer insertNamespace(String name, Integer parentId, Integer userId);
***REMOVED***
