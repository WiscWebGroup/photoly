package org.chengbing.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import java.io.Serializable;
import io.swagger.annotations.ApiModel;
import io.swagger.annotations.ApiModelProperty;
import lombok.Data;
import lombok.EqualsAndHashCode;

/**
 * <p>
 * 
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Data
@EqualsAndHashCode(callSuper = false)
@ApiModel(value="TagPhoto对象", description="")
public class TagPhoto implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "tp_id", type = IdType.AUTO)
    private Integer tpId;

    private Integer userId;

    private Integer tagId;

    private Integer photoId;

    public TagPhoto() {
    }

    public TagPhoto(Integer userId, Integer tagId, Integer photoId) {
        this.userId = userId;
        this.tagId = tagId;
        this.photoId = photoId;
    }
}
