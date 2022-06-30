package org.chengbing.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import java.io.Serializable;
import io.swagger.annotations.ApiModel;
import lombok.Data;
import lombok.EqualsAndHashCode;

***REMOVED***
 * <p>
 * 
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@Data
@EqualsAndHashCode(callSuper = false)
@ApiModel(value="GalleryPhoto对象", description="")
public class GalleryPhoto implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "gp_id", type = IdType.AUTO)
    private Integer gpId;

    private Integer gaId;

    private Integer photoId;

    public GalleryPhoto() {
***REMOVED***

    public GalleryPhoto(Integer gaId, Integer photoId) {
        this.gaId = gaId;
        this.photoId = photoId;
***REMOVED***
***REMOVED***
